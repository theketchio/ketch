package main

import (
	"bufio"
	"context"
	"errors"
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

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const appLogHelp = `
Show logs of an application
`

type appLogFn func(context.Context, config, appLogOptions, io.Writer) error

func newAppLogCmd(cfg config, out io.Writer, appLog appLogFn) *cobra.Command {
	options := appLogOptions{}
	cmd := &cobra.Command{
		Use:   "log APPNAME",
		Short: appLogHelp,
		Args:  cobra.ExactValidArgs(1),
		Long:  appLogHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]
			if !validation.ValidateName(options.appName) {
				return ErrInvalidAppName
			}
			return appLog(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.processName, "process", "p", "", "Process name.")
	cmd.Flags().IntVarP(&options.deploymentVersion, "version", "v", 0, "Deployment version.")
	cmd.Flags().BoolVarP(&options.follow, "follow", "f", false, "Specify if the logs should be streamed")
	cmd.Flags().BoolVar(&options.ignoreErrors, "ignore-errors", false, "if watching / following pod logs, allow for any errors that occur to be non-fatal")
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

func appLog(ctx context.Context, cfg config, options appLogOptions, out io.Writer) error {
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

	// FIXME: not implemented
	ignoreErrors bool
	timestamps   bool
	prefix       bool
	out          io.Writer
}

type readLogsFn func(client kubernetes.Interface, pod corev1.Pod) chan message
type streamLogsFn func(client kubernetes.Interface, pod corev1.Pod, lastTime time.Time, msgCh chan message) chan struct{}

func watchLogs(client kubernetes.Interface, options watchOptions, readLogs readLogsFn, streamLogs streamLogsFn) error {
	pods, err := client.CoreV1().Pods(options.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: options.selector.String()})
	if err != nil {
		return err
	}
	// we are going to read logs from all running pods, just read without streaming
	msgChs := make(map[types.UID]chan message, len(pods.Items))
	for _, pod := range pods.Items {
		msgChs[pod.UID] = readLogs(client, pod)
	}

	// we want to show the logs sorted by timestamp.
	// lets store one message per pod and then in a loop below we will select one message with minimal time on each iteration.
	messages := make(map[types.UID]message, len(msgChs))
	for podUID, msg := range msgChs {
		if m, ok := <-msg; ok {
			messages[podUID] = m
			continue
		}
		// the channel is closed - no logs
		delete(msgChs, podUID)
	}

	// we need a timestamp of the last message of each pod, so later we will stream logs from this time.
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
	watcher, err := client.CoreV1().Pods(options.namespace).Watch(context.TODO(), opts)
	if err != nil {
		return err
	}

	msgCh := make(chan message)
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
				doneChannels[pod.UID] = streamLogs(client, *pod, timeOfLastMessage[pod.UID], msgCh)
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

type message struct {
	time time.Time
	msg  string
	pod  *corev1.Pod
}

func (m message) Format(prefix bool, timestamps bool) string {
	var parts []string
	if prefix {
		// todo: containers
		parts = append(parts, fmt.Sprintf("[pod/%s]", m.pod.Name))
	}
	if timestamps {
		parts = append(parts, fmt.Sprintf("%v", m.time))
	}
	parts = append(parts, m.msg)
	return strings.Join(parts, " ")
}

func readString(reader *bufio.Reader, pod *corev1.Pod) (*message, error) {
	line, err := reader.ReadString('\n')
	if err == io.EOF {
		return nil, err
	}
	parts := strings.Split(line, " ")
	timestamp := parts[0]
	msg := line[len(timestamp):]
	messageTime, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return nil, err
	}
	return &message{msg: msg, time: messageTime, pod: pod}, nil
}

func ketchContainerName(pod corev1.Pod) (*string, error) {
	for _, c := range pod.Spec.Containers {
		// that's an application pod, it contains one ketch container.
		// we don't care about the other containers.
		if strings.HasPrefix(pod.Name, c.Name) {
			return &c.Name, nil
		}
	}
	return nil, errors.New("no ketch container")
}

// readLogs runs a goroutine that reads logs of the given pod. readLogs returns a message channel to receive logs.
// Once there are no more logs, readLogs closes the message channel.
func readLogs(client kubernetes.Interface, pod corev1.Pod) chan message {
	msgCh := make(chan message)
	go func() {
		defer func() {
			close(msgCh)
		}()
		containerName, err := ketchContainerName(pod)
		if err != nil {
			// FIXME: handle
			return
		}
		req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Timestamps: true, Container: *containerName})
		stream, err := req.Stream(context.TODO())
		if err != nil {
			return
		}
		reader := bufio.NewReader(stream)
		defer stream.Close()

		for {
			msg, err := readString(reader, &pod)
			if err == io.EOF {
				return
			}
			if err != nil {
				fmt.Printf("failed to read logs from pod %v: %v\n", pod.Name, err)
				return
			}
			msgCh <- *msg
		}
	}()
	return msgCh
}

// streamLogs runs a goroutine that streams logs of the given pod to the given message channel. streamLogs returns a channel used to stop the goroutine.
func streamLogs(client kubernetes.Interface, pod corev1.Pod, lastTime time.Time, msgCh chan message) chan struct{} {
	doneCh := make(chan struct{})
	go func() {
		for {
			errCh := make(chan error)
			go func() {
				sinceTime := metav1.NewTime(lastTime)
				containerName, err := ketchContainerName(pod)
				if err != nil {
					// FIXME: handle
					return
				}
				options := &corev1.PodLogOptions{
					Follow:     true,
					Timestamps: true,
					SinceTime:  &sinceTime,
					Container:  *containerName,
				}
				req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, options)
				stream, err := req.Stream(context.TODO())
				if err != nil {
					time.Sleep(500 * time.Millisecond)
					errCh <- err
					return
				}
				reader := bufio.NewReader(stream)
				defer stream.Close()

				for {
					msg, err := readString(reader, &pod)
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
			case _ = <-doneCh:
				return
			case err := <-errCh:
				if err != io.EOF {
					fmt.Printf("failed to read logs from pod %v: %v\n", pod.Name, err)
				}
			}
		}
	}()
	return doneCh
}
