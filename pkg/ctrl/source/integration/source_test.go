package integration

import (
	"fmt"

	"github.com/kubernetes-sigs/kubebuilder/pkg/ctrl/event"
	"github.com/kubernetes-sigs/kubebuilder/pkg/ctrl/eventhandler"
	"github.com/kubernetes-sigs/kubebuilder/pkg/ctrl/source"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
)

var _ = Describe("Source", func() {
	var instance1, instance2 source.KindSource
	var obj runtime.Object
	var q workqueue.RateLimitingInterface
	var c1, c2 chan interface{}
	var ns string
	count := 0

	BeforeEach(func() {
		// Create the namespace for the test
		ns = fmt.Sprintf("ctrl-source-kindsource-%v", count)
		count++
		_, err := clientset.CoreV1().Namespaces().Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		})
		Expect(err).NotTo(HaveOccurred())

		q = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "test")
		c1 = make(chan interface{})
		c2 = make(chan interface{})
	})

	JustBeforeEach(func() {
		instance1 = source.KindSource{Type: obj}
		instance1.InitInformerCache(icache)

		instance2 = source.KindSource{Type: obj}
		instance2.InitInformerCache(icache)
	})

	AfterEach(func() {
		err := clientset.CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
		close(c1)
		close(c2)
	})

	Describe("KindSource", func() {
		Context("for a Deployment resource", func() {
			obj = &appsv1.Deployment{}

			It("should provide Deployment Events", func(done Done) {
				var created, updated, deleted *appsv1.Deployment
				var err error

				// Get the client and Deployment used to create events
				client := clientset.AppsV1().Deployments(ns)
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "deployment-name"},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "nginx",
										Image: "nginx",
									},
								},
							},
						},
					},
				}

				// Create an event handler to verify the events
				newHandler := func(c chan interface{}) eventhandler.EventHandlerFuncs {
					return eventhandler.EventHandlerFuncs{
						CreateFunc: func(rli workqueue.RateLimitingInterface, evt event.CreateEvent) {
							defer GinkgoRecover()
							Expect(rli).To(Equal(q))
							c <- evt
						},
						UpdateFunc: func(rli workqueue.RateLimitingInterface, evt event.UpdateEvent) {
							defer GinkgoRecover()
							Expect(rli).To(Equal(q))
							c <- evt
						},
						DeleteFunc: func(rli workqueue.RateLimitingInterface, evt event.DeleteEvent) {
							defer GinkgoRecover()
							Expect(rli).To(Equal(q))
							c <- evt
						},
					}
				}
				handler1 := newHandler(c1)
				handler2 := newHandler(c2)

				// Create 2 instances
				instance1.Start(handler1, q)
				instance2.Start(handler2, q)

				By("Creating a Deployment and expecting the CreateEvent.")
				created, err = client.Create(deployment)
				Expect(err).NotTo(HaveOccurred())
				Expect(created).NotTo(BeNil())

				// Check first CreateEvent
				evt := <-c1
				createEvt, ok := evt.(event.CreateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.CreateEvent{}))
				objMeta, ok := createEvt.Meta.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", createEvt.Meta, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&created.ObjectMeta))
				Expect(createEvt.Object).To(Equal(created))

				// Check second CreateEvent
				evt = <-c2
				createEvt, ok = evt.(event.CreateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.CreateEvent{}))
				objMeta, ok = createEvt.Meta.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", createEvt.Meta, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&created.ObjectMeta))
				Expect(createEvt.Object).To(Equal(created))

				By("Updating a Deployment and expecting the UpdateEvent.")
				updated = created.DeepCopy()
				updated.Labels = map[string]string{"biz": "buz"}
				updated, err = client.Update(updated)
				Expect(err).NotTo(HaveOccurred())

				// Check first UpdateEvent
				evt = <-c1
				updateEvt, ok := evt.(event.UpdateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.UpdateEvent{}))

				objMeta, ok = updateEvt.MetaNew.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", updateEvt.MetaNew, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&updated.ObjectMeta))
				Expect(updateEvt.ObjectNew).To(Equal(updated))

				objMeta, ok = updateEvt.MetaOld.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", updateEvt.MetaOld, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&created.ObjectMeta))
				Expect(updateEvt.ObjectOld).To(Equal(created))

				// Check second UpdateEvent
				evt = <-c2
				updateEvt, ok = evt.(event.UpdateEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.UpdateEvent{}))

				objMeta, ok = updateEvt.MetaNew.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", updateEvt.MetaNew, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&updated.ObjectMeta))
				Expect(updateEvt.ObjectNew).To(Equal(updated))

				objMeta, ok = updateEvt.MetaOld.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", updateEvt.MetaOld, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&created.ObjectMeta))
				Expect(updateEvt.ObjectOld).To(Equal(created))

				By("Deleting a Deployment and expecting the Delete.")
				deleted = updated.DeepCopy()
				err = client.Delete(created.Name, &metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				deleted.SetResourceVersion("")
				evt = <-c1
				deleteEvt, ok := evt.(event.DeleteEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.DeleteEvent{}))
				deleteEvt.Meta.SetResourceVersion("")
				objMeta, ok = deleteEvt.Meta.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", deleteEvt.Meta, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&deleted.ObjectMeta))
				Expect(deleteEvt.Object).To(Equal(deleted))

				evt = <-c2
				deleteEvt, ok = evt.(event.DeleteEvent)
				Expect(ok).To(BeTrue(), fmt.Sprintf("expect %T to be %T", evt, event.DeleteEvent{}))
				deleteEvt.Meta.SetResourceVersion("")
				objMeta, ok = deleteEvt.Meta.(*metav1.ObjectMeta)
				Expect(ok).To(BeTrue(), fmt.Sprintf(
					"expect %T to be %T", deleteEvt.Meta, &metav1.ObjectMeta{}))
				Expect(objMeta).To(Equal(&deleted.ObjectMeta))
				Expect(deleteEvt.Object).To(Equal(deleted))

				close(done)
			}, 5)
		})

		// TODO: Write this test
		Context("for a Foo CRD resource", func() {
			It("should provide Foo Events", func() {

			})
		})
	})
})