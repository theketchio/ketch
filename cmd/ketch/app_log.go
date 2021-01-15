package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const (
	appLogHelp = `
Show logs of an application
`
	streamLogReconnectDelay = 500 * time.Millisecond
)

type appLogFn func(context.Context, config, appLogOptions, io.Writer, watchLogsFn) error

func newAppLogCmd(cfg config, out io.Writer, appLog appLogFn) *cobra.Command {
	options := appLogOptions{}
	cmd := &cobra.Command{
		Use:   "log APPNAME",
		Short: "Show logs of an application",
		Args:  cobra.ExactValidArgs(1),
		Long:  appLogHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]
			if !validation.ValidateName(options.appName) {
				return ErrInvalidAppName
			}
			return appLog(cmd.Context(), cfg, options, out, watchLogs)
		},
	}
	cmd.Flags().StringVarP(&options.processName, "process", "p", "", "Process name")
	cmd.Flags().IntVarP(&options.deploymentVersion, "version", "v", 0, "Deployment version")
	cmd.Flags().BoolVarP(&options.follow, "follow", "f", false, "Specify if the logs should be streamed")
	cmd.Flags().BoolVar(&options.ignoreErrors, "ignore-errors", false, "If watching / following pod logs, allow for any errors that occur to be non-fatal")
	cmd.Flags().BoolVar(&options.prefix, "prefix", false, "Prefix each log line with the log source (pod name and container name)")
	cmd.Flags().BoolVar(&options.timestamps, "timestamps", false, "Include timestamps on each line in the log output")

	return cmd
}

type appLogOptions struct {
	appName           string
	processName       string
	deploymentVersion int
	follow            bool
	ignoreErrors      bool
	timestamps        bool
	prefix            bool
}

type watchLogsFn func(client kubernetes.Interface, options watchOptions, readLogs readLogsFn, streamLogs streamLogsFn) error

func appLog(ctx context.Context, cfg config, options appLogOptions, out io.Writer, watchLogs watchLogsFn) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app instance: %w", err)
	}
	pool := ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, &pool); err != nil {
		return fmt.Errorf("failed to get pool instance: %w", err)
	}
	set := map[string]string{
		ketchAppNameLabel: options.appName,
	}
	if len(options.processName) > 0 {
		set[ketchProcessNameLabel] = options.processName
	}
	if options.deploymentVersion > 0 {
		set[ketchDeploymentVersionLabel] = fmt.Sprintf("%d", options.deploymentVersion)
	}
	s := labels.SelectorFromSet(set)
	opts := watchOptions{
		namespace:    pool.Spec.NamespaceName,
		selector:     s,
		follow:       options.follow,
		ignoreErrors: options.ignoreErrors,
		timestamps:   options.timestamps,
		prefix:       options.prefix,
		out:          out,
	}
	return watchLogs(cfg.KubernetesClient(), opts, readLogs, streamLogs)
}

type watchOptions struct {
	namespace    string
	selector     labels.Selector
	follow       bool
	ignoreErrors bool
	timestamps   bool
	prefix       bool
	out          io.Writer
}

// ketchContainerName returns a name of an application container.
// A pod can have several containers, one of them is defined and created by ketch, it's an application container.
// The others can be injected by istio, vault, etc.
func ketchContainerName(pod corev1.Pod) (*string, error) {
	// this is an application pod.
	// it has one app container, we don't care about the other containers.
	for _, c := range pod.Spec.Containers {
		if strings.HasPrefix(pod.Name, c.Name) {
			return &c.Name, nil
		}
	}
	return nil, fmt.Errorf("pod %s doesn't have an app container", pod.Name)
}

func isContainerRunning(pod corev1.Pod, containerName string) bool {
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == containerName {
			return container.State.Running != nil
		}
	}
	return false
}

type readLogsFn func(getLogs getLogsFn, pod corev1.Pod, containerName string, out io.Writer) chan logMessage
type streamLogsFn func(getLogs getLogsFn, pod corev1.Pod, containerName string, out io.Writer, lastTime time.Time, msgCh chan logMessage) chan struct{}

func watchLogs(cli kubernetes.Interface, options watchOptions, readLogs readLogsFn, streamLogs streamLogsFn) error {
	pods, err := cli.CoreV1().Pods(options.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: options.selector.String()})
	if err != nil {
		return err
	}
	// we are going to read logs from all running pods, just read without streaming.
	msgChs := make(map[types.UID]chan logMessage, len(pods.Items))
	for _, pod := range pods.Items {
		containerName, err := ketchContainerName(pod)
		if err != nil {
			return err
		}
		msgChs[pod.UID] = readLogs(cli.CoreV1().Pods(pod.Namespace).GetLogs, pod, *containerName, options.out)
	}

	// we want to show the logs sorted by timestamp.
	// lets store one message per pod and then in a loop below we will select one message with minimal time on each iteration.
	messages := make(map[types.UID]logMessage, len(msgChs))
	for podUID, msg := range msgChs {
		if m, ok := <-msg; ok {
			messages[podUID] = m
			continue
		}
		// the channel is closed - no logs
		delete(msgChs, podUID)
	}

	// we need a timestamp of the last message of each pod, so later we will stream logs from this time.
	// we avoid using sort.Sort because downloading all logs and keeping them in memory can be resource-consuming operation.
	timeOfLastMessage := make(map[types.UID]time.Time)
	for {
		// on each iteration we are looking for a message with minimal time
		if len(messages) == 0 {
			break
		}
		var target types.UID
		for name, m := range messages {
			if len(target) == 0 || messages[target].time.After(m.time) {
				target = name
			}
		}
		m := messages[target]
		timeOfLastMessage[target] = m.time

		fmt.Fprintf(options.out, "%s", m.Format(options.prefix, options.timestamps))

		m, ok := <-msgChs[target]
		if !ok {
			delete(msgChs, target)
			delete(messages, target)
			continue
		}
		messages[target] = m
	}

	if !options.follow {
		return nil
	}

	opts := metav1.ListOptions{
		Watch:         true,
		LabelSelector: options.selector.String(),
	}
	watcher, err := cli.CoreV1().Pods(options.namespace).Watch(context.TODO(), opts)
	if err != nil {
		return err
	}

	msgCh := make(chan logMessage)
	doneChannels := make(map[types.UID]chan struct{})

	for {
		select {
		case e := <-watcher.ResultChan():
			if e.Object == nil {
				// channel is closed
				return nil
			}
			pod := e.Object.(*corev1.Pod)
			switch e.Type {
			case watch.Added, watch.Modified:
				if _, ok := doneChannels[pod.UID]; ok {
					continue
				}
				containerName, err := ketchContainerName(*pod)
				if err != nil {
					if !options.ignoreErrors {
						return err
					}
					continue
				}
				if !isContainerRunning(*pod, *containerName) {
					continue
				}
				logs := cli.CoreV1().Pods(pod.Namespace).GetLogs
				doneChannels[pod.UID] = streamLogs(logs, *pod, *containerName, options.out, timeOfLastMessage[pod.UID], msgCh)

			case watch.Deleted:
				if doneCh, ok := doneChannels[pod.UID]; ok {
					doneCh <- struct{}{}
					delete(doneChannels, pod.UID)
				}
			}
		case m := <-msgCh:
			fmt.Fprintf(options.out, "%s", m.Format(options.prefix, options.timestamps))
		}
	}
}

type logMessage struct {
	time          time.Time
	msg           string
	pod           *corev1.Pod
	containerName string
}

// Format returns a string representation of the logMessage.
func (m logMessage) Format(prefix bool, timestamps bool) string {
	var parts []string
	if timestamps {
		parts = append(parts, fmt.Sprintf("%v", m.time.Format(time.RFC3339Nano)))
	}
	if prefix {
		parts = append(parts, fmt.Sprintf("[%s/%s]", m.pod.Name, m.containerName))
	}
	parts = append(parts, m.msg)
	return strings.Join(parts, " ")
}

func readString(reader *bufio.Reader, pod *corev1.Pod, containerName string) (*logMessage, error) {
	line, err := reader.ReadString('\n')
	if len(line) > 0 {
		parts := strings.Split(line, " ")
		timestamp := parts[0]
		msg := line[len(timestamp)+1:]
		messageTime, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return nil, ErrLogUnknownTimeFormat
		}
		return &logMessage{msg: msg, time: messageTime, pod: pod, containerName: containerName}, nil
	}
	return nil, err
}

type getLogsFn func(name string, opts *corev1.PodLogOptions) *restclient.Request

// readLogs runs a goroutine that reads logs of the given pod. readLogs returns a message channel to receive logs.
// Once there are no more logs, readLogs closes the message channel.
func readLogs(getLogs getLogsFn, pod corev1.Pod, containerName string, out io.Writer) chan logMessage {
	msgCh := make(chan logMessage)
	go func() {
		defer func() {
			close(msgCh)
		}()
		req := getLogs(pod.Name, &corev1.PodLogOptions{Timestamps: true, Container: containerName})
		stream, err := req.Stream(context.TODO())
		if err != nil {
			fmt.Fprintf(out, "failed to read logs from pod %v: %v\n", pod.Name, unwrappedError(err).Error())
			return
		}
		reader := bufio.NewReader(stream)
		defer stream.Close()

		for {
			msg, err := readString(reader, &pod, containerName)
			if err == io.EOF {
				return
			}
			if err != nil {
				fmt.Fprintf(out, "failed to read logs from pod %v: %v\n", pod.Name, err)
				return
			}
			msgCh <- *msg
		}
	}()
	return msgCh
}

// streamLogs runs a goroutine that streams logs of the desired container of the given pod and to the given message channel.
// streamLogs returns a channel used to stop the goroutine.
func streamLogs(getLogs getLogsFn, pod corev1.Pod, containerName string, out io.Writer, lastTime time.Time, msgCh chan logMessage) chan struct{} {
	doneCh := make(chan struct{})
	go func() {
		for {
			errCh := make(chan error)
			go func() {
				sinceTime := metav1.NewTime(lastTime)
				options := &corev1.PodLogOptions{
					Follow:     true,
					Timestamps: true,
					SinceTime:  &sinceTime,
					Container:  containerName,
				}
				req := getLogs(pod.Name, options)
				stream, err := req.Stream(context.TODO())
				if err != nil {
					time.Sleep(streamLogReconnectDelay)
					errCh <- unwrappedError(err)
					return
				}
				reader := bufio.NewReader(stream)
				defer stream.Close()

				for {
					msg, err := readString(reader, &pod, containerName)
					if err != nil {
						errCh <- err
						return
					}
					if msg.time.Before(lastTime) || msg.time.Equal(lastTime) {
						continue
					}
					lastTime = msg.time
					msgCh <- *msg
				}
			}()

			select {
			case <-doneCh:
				return
			case err := <-errCh:
				if err != io.EOF {
					fmt.Fprintf(out, "failed to read logs from pod %v: %v\n", pod.Name, err)
				}
			}
		}
	}()
	return doneCh
}
