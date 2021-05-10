package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	testclient "k8s.io/client-go/testing"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_watchLogs(t *testing.T) {
	startDate := time.Date(2021, 1, 13, 16, 49, 0, 1, time.UTC)
	readLogsLocal := func(_ getLogsFn, pod corev1.Pod, contName string, _ io.Writer) chan logMessage {
		ch := make(chan logMessage)
		startDate, err := time.Parse(time.RFC3339Nano, pod.Labels["TIME"])
		require.Nil(t, err)
		go func() {
			defer func() {
				close(ch)
			}()
			if !isContainerRunning(pod, contName) {
				return
			}
			msgs := []logMessage{
				{time: startDate.Add(0 * time.Second), msg: fmt.Sprintf("%s 0\n", contName), pod: &pod, containerName: contName},
				{time: startDate.Add(5 * time.Second), msg: fmt.Sprintf("%s 1\n", contName), pod: &pod, containerName: contName},
				{time: startDate.Add(10 * time.Second), msg: fmt.Sprintf("%s 2\n", contName), pod: &pod, containerName: contName},
				{time: startDate.Add(15 * time.Second), msg: fmt.Sprintf("%s 3\n", contName), pod: &pod, containerName: contName},
			}
			for _, msg := range msgs {
				ch <- msg
			}
		}()
		return ch
	}

	streamLogsLocal := func(_ getLogsFn, pod corev1.Pod, contName string, out io.Writer, lastTime time.Time, msgCh chan logMessage) chan struct{} {
		doneCh := make(chan struct{})
		go func() {
			msgs := []logMessage{
				{time: startDate.Add(0 * time.Second), msg: fmt.Sprintf("%s stream 0\n", contName), pod: &pod, containerName: contName},
				{time: startDate.Add(5 * time.Second), msg: fmt.Sprintf("%s stream 1\n", contName), pod: &pod, containerName: contName},
				{time: startDate.Add(10 * time.Second), msg: fmt.Sprintf("%s stream 2\n", contName), pod: &pod, containerName: contName},
				{time: startDate.Add(15 * time.Second), msg: fmt.Sprintf("%s stream 3\n", contName), pod: &pod, containerName: contName},
			}
			for _, msg := range msgs {
				msgCh <- msg
			}
			<-doneCh
		}()
		return doneCh
	}
	createPod := func(namespace string, name string, containers map[string]bool, logsStartTime time.Time) *corev1.Pod {
		parts := strings.Split(name, "-")
		appName, processName, version := parts[0], parts[1], parts[2]
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				UID:       types.UID(name),
				Labels: map[string]string{
					"TIME":                      logsStartTime.Format(time.RFC3339Nano),
					ketchAppNameLabel:           appName,
					ketchProcessNameLabel:       processName,
					ketchDeploymentVersionLabel: version,
				},
			},
		}
		for containerName, isRunning := range containers {
			pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: containerName})
			status := corev1.ContainerStatus{Name: containerName, State: corev1.ContainerState{}}
			if isRunning {
				status.State.Running = &corev1.ContainerStateRunning{}
			}
			pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, status)
		}
		return pod
	}
	tests := []struct {
		description        string
		options            watchOptions
		pods               []*corev1.Pod
		watcherHelper      func(watcher *watch.FakeWatcher, pods []*corev1.Pod)
		wantErr            string
		wantOutputFilename string
	}{
		{
			description: "happy path - several containers, no prefix, no timestamps",
			options: watchOptions{
				namespace: "default",
				selector:  labels.Everything(),
				follow:    false,
				prefix:    false,
			},
			pods: []*corev1.Pod{
				createPod("default", "dashboard-worker-2-random", map[string]bool{"dashboard-worker-2": true}, startDate.Add(2*time.Second)),
				createPod("default", "dashboard-worker-3-random", map[string]bool{"dashboard-worker-3": true}, startDate.Add(3*time.Second)),
				createPod("default", "hello-web-1-random", map[string]bool{"hello-web-1": true}, startDate),
				createPod("default", "hello-web-2-random", map[string]bool{"hello-web-2": true}, startDate.Add(time.Second)),
			},
			wantOutputFilename: "./testdata/app-log/1.output",
		},
		{
			description: "happy path - several containers, + prefix, no timestamps",
			options: watchOptions{
				namespace: "default",
				selector:  labels.Everything(),
				prefix:    true,
			},
			pods: []*corev1.Pod{
				createPod("default", "dashboard-worker-2-random", map[string]bool{"dashboard-worker-2": true}, startDate.Add(2*time.Second)),
				createPod("default", "dashboard-worker-3-random", map[string]bool{"dashboard-worker-3": true}, startDate.Add(3*time.Second)),
				createPod("default", "hello-web-1-random", map[string]bool{"hello-web-1": true}, startDate),
				createPod("default", "hello-web-2-random", map[string]bool{"hello-web-2": true}, startDate.Add(time.Second)),
			},
			wantOutputFilename: "./testdata/app-log/2.output",
		},
		{
			description: "happy path - several containers, + prefix, + timestamps",
			options: watchOptions{
				namespace:  "default",
				selector:   labels.Everything(),
				prefix:     true,
				timestamps: true,
			},
			pods: []*corev1.Pod{
				createPod("default", "dashboard-worker-2-random", map[string]bool{"dashboard-worker-2": true}, startDate.Add(1*time.Second)),
				createPod("default", "dashboard-worker-3-random", map[string]bool{"dashboard-worker-3": true}, startDate.Add(2*time.Second)),
				createPod("default", "hello-web-1-random", map[string]bool{"hello-web-1": true}, startDate),
				createPod("default", "hello-web-2-random", map[string]bool{"hello-web-2": true}, startDate.Add(3*time.Second)),
			},
			wantOutputFilename: "./testdata/app-log/3.output",
		},
		{
			description: "happy path - logs from dashboard containers, + prefix, + timestamps",
			options: watchOptions{
				namespace:  "default",
				selector:   labels.SelectorFromSet(map[string]string{ketchAppNameLabel: "dashboard"}),
				prefix:     true,
				timestamps: true,
			},
			pods: []*corev1.Pod{
				createPod("default", "dashboard-worker-2-random", map[string]bool{"dashboard-worker-2": true}, startDate.Add(1*time.Second)),
				createPod("default", "dashboard-worker-3-random", map[string]bool{"dashboard-worker-3": true}, startDate.Add(2*time.Second)),
				createPod("default", "hello-web-1-random", map[string]bool{"hello-web-1": true}, startDate),
				createPod("default", "hello-web-2-random", map[string]bool{"hello-web-2": true}, startDate.Add(3*time.Second)),
			},
			wantOutputFilename: "./testdata/app-log/4.output",
		},
		{
			description: "happy path - logs from dashboard containers, + prefix, + timestamps",
			options: watchOptions{
				namespace: "default",
				selector:  labels.Everything(),
			},
			pods: []*corev1.Pod{
				createPod("default", "hello-web-1-random", map[string]bool{"istio-proxy": true}, startDate),
			},
			wantErr: "pod hello-web-1-random doesn't have an app container",
		},
		{
			description: "happy path with streaming: prefix + timestamps",
			options: watchOptions{
				namespace:  "default",
				selector:   labels.Everything(),
				follow:     true,
				prefix:     true,
				timestamps: true,
			},
			pods: []*corev1.Pod{
				createPod("default", "hello-web-1-random", map[string]bool{"hello-web-1": true}, startDate),
				createPod("default", "hello-web-2-random", map[string]bool{"hello-web-2": true}, startDate.Add(time.Second)),
			},
			watcherHelper: func(watcher *watch.FakeWatcher, pods []*corev1.Pod) {
				for _, pod := range pods {
					watcher.Add(pod)
					time.Sleep(1 * time.Second)
				}
				watcher.Stop()
			},
			wantOutputFilename: "./testdata/app-log/stream-1.output",
		},
		{
			description: "happy path with streaming",
			options: watchOptions{
				namespace: "default",
				selector:  labels.Everything(),
				follow:    true,
			},
			pods: []*corev1.Pod{
				createPod("default", "hello-web-1-random", map[string]bool{"hello-web-1": true}, startDate),
				createPod("default", "hello-web-2-random", map[string]bool{"hello-web-2": true}, startDate.Add(time.Second)),
			},
			watcherHelper: func(watcher *watch.FakeWatcher, pods []*corev1.Pod) {
				for _, pod := range pods {
					watcher.Add(pod)
					watcher.Add(pod)
					watcher.Modify(pod)
					watcher.Modify(pod)
					time.Sleep(1 * time.Second)
					watcher.Delete(pod)
					watcher.Delete(pod)
				}
				watcher.Stop()
			},
			wantOutputFilename: "./testdata/app-log/stream-2.output",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var objects []runtime.Object
			for _, pod := range tt.pods {
				objects = append(objects, pod)
			}
			kubeClient := fake.NewSimpleClientset(objects...)
			if tt.watcherHelper != nil {
				watcher := watch.NewFake()
				kubeClient.PrependWatchReactor("pods", testclient.DefaultWatchReactor(watcher, nil))
				go tt.watcherHelper(watcher, tt.pods)
			}

			out := &bytes.Buffer{}
			tt.options.out = out
			err := watchLogs(kubeClient, tt.options, readLogsLocal, streamLogsLocal)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)

			wantOut, err := ioutil.ReadFile(tt.wantOutputFilename)
			require.Nil(t, err)
			require.Equal(t, strings.TrimSpace(out.String()), strings.TrimSpace(string(wantOut)))
		})
	}
}

func Test_appLog(t *testing.T) {
	dashboard := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: ketchv1.AppSpec{
			Pool: "gke",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
			},
		},
	}
	gke := &ketchv1.Pool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gke",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				IngressType: ketchv1.IstioIngressControllerType,
			},
		},
	}
	tests := []struct {
		description      string
		cfg              config
		options          appLogOptions
		wantErr          string
		wantCalled       bool
		wantWatchOptions watchOptions
	}{
		{
			description: "happy path, app selector + prefix",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, gke},
			},
			options:    appLogOptions{appName: "dashboard", prefix: true},
			wantCalled: true,
			wantWatchOptions: watchOptions{
				namespace: "ketch-gke",
				selector: labels.SelectorFromSet(map[string]string{
					ketchAppNameLabel: "dashboard",
				}),
				prefix: true,
			},
		},
		{
			description: "happy path, app selector + follow",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, gke},
			},
			options:    appLogOptions{appName: "dashboard", follow: true},
			wantCalled: true,
			wantWatchOptions: watchOptions{
				namespace: "ketch-gke",
				selector: labels.SelectorFromSet(map[string]string{
					ketchAppNameLabel: "dashboard",
				}),
				follow: true,
			},
		},
		{
			description: "happy path: app and process selector + timestamps",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, gke},
			},
			options:    appLogOptions{appName: "dashboard", processName: "web", timestamps: true},
			wantCalled: true,
			wantWatchOptions: watchOptions{
				namespace: "ketch-gke",
				selector: labels.SelectorFromSet(map[string]string{
					ketchAppNameLabel:     "dashboard",
					ketchProcessNameLabel: "web",
				}),
				timestamps: true,
			},
		},
		{
			description: "happy path: app + process + deployment version selector + ignoreErrors",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, gke},
			},
			options:    appLogOptions{appName: "dashboard", processName: "worker", deploymentVersion: 4, ignoreErrors: true},
			wantCalled: true,
			wantWatchOptions: watchOptions{
				namespace: "ketch-gke",
				selector: labels.SelectorFromSet(map[string]string{
					ketchAppNameLabel:           "dashboard",
					ketchProcessNameLabel:       "worker",
					ketchDeploymentVersionLabel: "4",
				}),
				ignoreErrors: true,
			},
		},
		{
			description: "no app",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{gke},
			},
			options: appLogOptions{appName: "dashboard"},
			wantErr: `failed to get app instance: apps.theketch.io "dashboard" not found`,
		},
		{
			description: "no pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard},
			},
			options: appLogOptions{appName: "dashboard"},
			wantErr: `failed to get pool instance: pools.theketch.io "gke" not found`,
		},
		{
			description: "error from watchLog",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, gke},
			},
			options:    appLogOptions{appName: "dashboard"},
			wantErr:    `error from watchLog`,
			wantCalled: true,
			wantWatchOptions: watchOptions{
				namespace: "ketch-gke",
				selector: labels.SelectorFromSet(map[string]string{
					ketchAppNameLabel: "dashboard",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			out := &bytes.Buffer{}
			called := false
			watchFn := func(client kubernetes.Interface, options watchOptions, readLogs_ readLogsFn, streamLogs_ streamLogsFn) error {
				called = true
				options.out = nil
				require.Equal(t, tt.wantWatchOptions, options)
				if len(tt.wantErr) > 0 {
					return errors.New(tt.wantErr)
				}
				return nil
			}
			err := appLog(context.Background(), tt.cfg, tt.options, out, watchFn)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantCalled, called)
		})
	}
}

func Test_newAppLogCmd(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)
	tests := []struct {
		description string
		args        []string
		appLog      appLogFn
		wantErr     bool
		wantOptions appLogOptions
	}{
		{
			description: "happy path: follow",
			args:        []string{"ketch", "foo-bar", "-f"},
			appLog: func(ctx context.Context, c config, options appLogOptions, writer io.Writer, fn watchLogsFn) error {
				require.Equal(t, appLogOptions{follow: true, appName: "foo-bar"}, options)
				return nil
			},
		},
		{
			description: "happy path: follow long",
			args:        []string{"ketch", "foo-bar", "--follow=true"},
			appLog: func(ctx context.Context, c config, options appLogOptions, writer io.Writer, fn watchLogsFn) error {
				require.Equal(t, appLogOptions{follow: true, appName: "foo-bar"}, options)
				return nil
			},
		},
		{
			description: "happy path: ignore-errors",
			args:        []string{"ketch", "foo-bar", "--ignore-errors=true"},
			appLog: func(ctx context.Context, c config, options appLogOptions, writer io.Writer, fn watchLogsFn) error {
				require.Equal(t, appLogOptions{ignoreErrors: true, appName: "foo-bar"}, options)
				return nil
			},
		},
		{
			description: "happy path: prefix",
			args:        []string{"ketch", "foo-bar", "--prefix=true"},
			appLog: func(ctx context.Context, c config, options appLogOptions, writer io.Writer, fn watchLogsFn) error {
				require.Equal(t, appLogOptions{prefix: true, appName: "foo-bar"}, options)
				return nil
			},
		},
		{
			description: "happy path: timestamps",
			args:        []string{"ketch", "foo-bar", "--timestamps=true"},
			appLog: func(ctx context.Context, c config, options appLogOptions, writer io.Writer, fn watchLogsFn) error {
				require.Equal(t, appLogOptions{timestamps: true, appName: "foo-bar"}, options)
				return nil
			},
		},
		{
			description: "happy path: deployment version",
			args:        []string{"ketch", "dashboard", "--version=8"},
			appLog: func(ctx context.Context, c config, options appLogOptions, writer io.Writer, fn watchLogsFn) error {
				require.Equal(t, appLogOptions{deploymentVersion: 8, appName: "dashboard"}, options)
				return nil
			},
		},
		{
			description: "bad app name",
			args:        []string{"ketch", "_._"},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			os.Args = tt.args
			cmd := newAppLogCmd(nil, nil, tt.appLog)
			err := cmd.Execute()
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
		})
	}
}

func Test_readLogs(t *testing.T) {
	startDate := time.Date(2021, 1, 13, 16, 49, 0, 0, time.UTC)
	tests := []struct {
		description   string
		pod           corev1.Pod
		containerName string
		logs          []string
		wantMsgs      []string
		wantOut       string
	}{
		{
			description: "happy path",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello-web-1-random",
					Namespace: "default",
				},
			},
			containerName: "hello-web-1",
			logs: []string{
				fmt.Sprintf("%s message 1\n", startDate.Format(time.RFC3339Nano)),
				fmt.Sprintf("%s another long message\n", startDate.Add(2*time.Minute).Format(time.RFC3339Nano)),
			},
			wantMsgs: []string{
				fmt.Sprintf("%s message 1\n", startDate.Format(time.RFC3339Nano)),
				fmt.Sprintf("%s another long message\n", startDate.Add(2*time.Minute).Format(time.RFC3339Nano)),
			},
		},
		{
			description: "streaming error",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello-web-1-random",
					Namespace: "default",
				},
			},
			containerName: "hello-web-1",
			wantOut:       "failed to read logs from pod hello-web-1-random: some error\n",
		},
		{
			description: "parsing error",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello-web-1-random",
					Namespace: "default",
				},
			},
			containerName: "hello-web-1",
			logs: []string{
				fmt.Sprintf("%s message 1\n", startDate.Format(time.RFC1123)),
				fmt.Sprintf("%s another long message\n", startDate.Add(2*time.Minute).Format(time.RFC3339Nano)),
			},
			wantOut: "failed to read logs from pod hello-web-1-random: unknown time format\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			getLogs := func(name string, opts *corev1.PodLogOptions) *restclient.Request {
				require.Equal(t, true, opts.Timestamps)
				require.Equal(t, false, opts.Follow)
				require.Equal(t, tt.containerName, opts.Container)
				fakeClient := &fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						if len(tt.logs) == 0 {
							return nil, errors.New("some error")
						}
						resp := &http.Response{
							StatusCode: http.StatusOK,
							Body:       ioutil.NopCloser(strings.NewReader(strings.Join(tt.logs, ""))),
						}
						return resp, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
					GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
					VersionedAPIPath:     fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log", "default", name),
				}
				return fakeClient.Request()
			}
			out := &bytes.Buffer{}
			got := readLogs(getLogs, tt.pod, tt.containerName, out)
			var msgs []string
			for msg := range got {
				require.Equal(t, tt.pod, *msg.pod)
				require.Equal(t, tt.containerName, msg.containerName)
				msgs = append(msgs, fmt.Sprintf("%s %s", msg.time.Format(time.RFC3339Nano), msg.msg))
			}
			require.Equal(t, tt.wantMsgs, msgs)
			require.Equal(t, tt.wantOut, out.String())
		})
	}
}

func Test_streamLogs(t *testing.T) {
	startDate := time.Date(2021, 1, 13, 16, 49, 0, 0, time.UTC)
	tests := []struct {
		description   string
		pod           corev1.Pod
		containerName string
		ignoreErrors  bool
		logs          []string
		wantMsgs      []string
		wantOut       string
	}{
		{
			description: "happy path",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello-web-1-random",
					Namespace: "default",
				},
			},
			containerName: "hello-web-1",
			logs: []string{
				fmt.Sprintf("%s message 1\n", startDate.Format(time.RFC3339Nano)),
				fmt.Sprintf("%s another long message\n", startDate.Add(2*time.Minute).Format(time.RFC3339Nano)),
			},
			wantMsgs: []string{
				fmt.Sprintf("%s message 1\n", startDate.Format(time.RFC3339Nano)),
				fmt.Sprintf("%s another long message\n", startDate.Add(2*time.Minute).Format(time.RFC3339Nano)),
			},
		},
		{
			description: "streaming error",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello-web-1-random",
					Namespace: "default",
				},
			},
			containerName: "hello-web-1",
			wantOut:       "failed to read logs from pod hello-web-1-random: some error\n",
		},
		{
			description: "parsing error",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello-web-1-random",
					Namespace: "default",
				},
			},
			containerName: "hello-web-1",
			logs: []string{
				fmt.Sprintf("%s message 1\n", startDate.Format(time.RFC1123)),
				fmt.Sprintf("%s another long message\n", startDate.Add(2*time.Minute).Format(time.RFC3339Nano)),
			},
			wantMsgs: []string{
				fmt.Sprintf("%s another long message\n", startDate.Add(2*time.Minute).Format(time.RFC3339Nano)),
			},
			wantOut: "failed to read logs from pod hello-web-1-random: unknown time format\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			currentMsg := 0
			getLogs := func(name string, opts *corev1.PodLogOptions) *restclient.Request {
				require.Equal(t, true, opts.Timestamps)
				require.Equal(t, true, opts.Follow)
				require.Equal(t, tt.containerName, opts.Container)
				fakeClient := &fakerest.RESTClient{
					Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
						if len(tt.logs) == 0 {
							return nil, errors.New("some error")
						}
						if currentMsg < len(tt.logs) {
							currentMsg += 1
							return &http.Response{
								StatusCode: http.StatusOK,
								Body:       ioutil.NopCloser(strings.NewReader(tt.logs[currentMsg-1])),
							}, nil
						}
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Body:       ioutil.NopCloser(strings.NewReader("")),
						}, nil
					}),
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
					GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
					VersionedAPIPath:     fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log", "default", name),
				}
				return fakeClient.Request()
			}
			out := &bytes.Buffer{}
			msgCh := make(chan logMessage)
			doneCh := streamLogs(getLogs, tt.pod, tt.containerName, out, time.Time{}, msgCh)
			require.NotNil(t, doneCh)
			var msgs []string
			ch := time.After(1 * time.Second)
			for {
				select {
				case msg := <-msgCh:
					require.Equal(t, tt.pod, *msg.pod)
					require.Equal(t, tt.containerName, msg.containerName)
					msgs = append(msgs, fmt.Sprintf("%s %s", msg.time.Format(time.RFC3339Nano), msg.msg))
				case <-ch:
					doneCh <- struct{}{}
					require.Equal(t, tt.wantMsgs, msgs)
					// if the error case occurs the code can loop through and write multiple identical error
					// messages, based on timing, we're only interested in the first message, so this avoids
					// a flaky test
					firstMessage := strings.SplitAfter(out.String(), "\n")[0]
					require.Equal(t, tt.wantOut, firstMessage)
					return
				}
			}
		})
	}
}
