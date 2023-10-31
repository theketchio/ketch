/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/chart"
	"github.com/theketchio/ketch/internal/controllers"
	"github.com/theketchio/ketch/internal/templates"
	"github.com/theketchio/ketch/internal/watchers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func main() {

	var metricsAddr string
	var enableLeaderElection bool
	var disableWebhooks bool
	var group string
	var namespace string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&disableWebhooks, "disable-webhooks", false, "Disable webhooks.")
	flag.StringVar(&group, "group", ketchv1.TheKetchGroup, "specify a non-default group")
	flag.StringVar(&namespace, "namespace", controllers.KetchNamespace, "specify a non-default namespace")
	flag.Parse()

	_ = clientgoscheme.AddToScheme(scheme)
	_ = ketchv1.AddToScheme(ketchv1.WithGroup(group))(scheme)
	// +kubebuilder:scaffold:scheme

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Metrics:          metricsserver.Options{BindAddress: metricsAddr},
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "dcbf0335.theketch.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	storageClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create storage client")
		os.Exit(1)
	}
	// Storage uses its own client.Client
	// because mgr.GetClient() returns a client that requires some time to initialize its internal cache,
	// and storage.Update() operation fails.
	storage := templates.NewStorage(storageClient, namespace)
	if err = storage.Update(templates.IngressConfigMapName(ketchv1.TraefikIngressControllerType.String()), templates.TraefikDefaultTemplates); err != nil {
		setupLog.Error(err, "unable to set default templates")
		os.Exit(1)
	}
	if err = storage.Update(templates.IngressConfigMapName(ketchv1.IstioIngressControllerType.String()), templates.IstioDefaultTemplates); err != nil {
		setupLog.Error(err, "unable to set default templates")
		os.Exit(1)
	}
	if err = storage.Update(templates.IngressConfigMapName(ketchv1.NginxIngressControllerType.String()), templates.NginxDefaultTemplates); err != nil {
		setupLog.Error(err, "unable to set default templates")
		os.Exit(1)
	}
	if err = storage.Update(templates.JobConfigMapName(), templates.JobTemplates); err != nil {
		setupLog.Error(err, "unable to set default templates")
		os.Exit(1)
	}
	if err = storage.Update(templates.CronJobConfigMapName(), templates.CronJobTemplates); err != nil {
		setupLog.Error(err, "unable to set default templates")
		os.Exit(1)
	}

	logg := ctrl.Log.WithName("controllers").WithName("App")

	clientSet, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		setupLog.Error(err, "unable to initialize clientset")
		os.Exit(1)
	}
	eventBroadcaster := record.NewBroadcasterWithCorrelatorOptions(record.CorrelatorOptions{
		BurstSize: math.MaxInt,
		QPS:       1,
	})
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) { logg.Info(fmt.Sprintf(format, args...)) })
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})

	factory := chart.NewHelmClientFactory()

	if err = (&controllers.AppReconciler{
		TemplateReader: storage,
		Client:         mgr.GetClient(),
		Log:            logg,
		Scheme:         mgr.GetScheme(),
		HelmFactoryFn: func(namespace string) (controllers.Helm, error) {
			return factory.NewHelmClient(namespace, mgr.GetClient(), logg)
		},
		Now:   time.Now,
		Group: group,
		Recorder: eventBroadcaster.NewRecorder(clientgoscheme.Scheme, v1.EventSource{
			Component: "ketch-controller",
		},
		),
		Config:    ctrl.GetConfigOrDie(),
		CancelMap: controllers.NewCancelMap(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "App")
		os.Exit(1)
	}

	if err = (&controllers.JobReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("Job"),
		Scheme:         mgr.GetScheme(),
		TemplateReader: storage,
		HelmFactoryFn: func(namespace string) (controllers.Helm, error) {
			return factory.NewHelmClient(namespace, mgr.GetClient(), logg)
		},
		Recorder: eventBroadcaster.NewRecorder(clientgoscheme.Scheme, v1.EventSource{
			Component: "ketch-controller",
		}),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Job")
		os.Exit(1)
	}

	if !disableWebhooks {
		if err = (&ketchv1.Job{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Job")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err = watchers.NewIngressWatcher(clientSet, mgr.GetClient(), logg).Inform(ctx); err != nil {
		setupLog.Error(err, "unable to create ingress watcher", "watcher", "ingress")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
