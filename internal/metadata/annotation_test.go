package metadata_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalmetadata "github.com/rabbitmq/cluster-operator/internal/metadata"
)

var _ = Describe("Annotation", func() {
	defaultOne := map[string]string{"foo": "bar"}
	defaultAnnotationsWithK8s := map[string]string{
		"kubernetes.io.instance": "i1",
		"k8s.io.namespace":       "ns",
		"another-annotation":     "some-value",
	}

	DescribeTable("Reconcile annotations",
		func(expectedAnnotations map[string]string, existingAnnotations map[string]string, defaultAnnotations ...map[string]string) {
			reconciledAnnotations := internalmetadata.ReconcileAnnotations(existingAnnotations, defaultAnnotations...)
			Expect(reconciledAnnotations).To(Equal(expectedAnnotations))
		},

		Entry("existingAnnotations is nil return merged default annotations",
			map[string]string{
				"foo":                    "bar",
				"kubernetes.io.instance": "i1",
				"k8s.io.namespace":       "ns",
				"another-annotation":     "some-value",
			}, nil, defaultOne, defaultAnnotationsWithK8s),

		Entry("no default annotations are present returns existing annotations",
			map[string]string{
				"existingAnnotation": "value",
				"k8s.io.annotation":  "annot",
			},
			map[string]string{
				"existingAnnotation": "value",
				"k8s.io.annotation":  "annot",
			}),

		Entry("merges default and existing annotations",
			map[string]string{
				"existingAnnotation":     "value",
				"k8s.io.annotation":      "annot",
				"foo":                    "bar",
				"kubernetes.io.instance": "i1",
				"k8s.io.namespace":       "ns",
				"another-annotation":     "some-value",
			},
			map[string]string{
				"existingAnnotation": "value",
				"k8s.io.annotation":  "annot",
			}, defaultOne, defaultAnnotationsWithK8s),
	)

	DescribeTable("Reconcile and filter annotations",
		func(expectedAnnotations map[string]string, existingAnnotations map[string]string, defaultAnnotations ...map[string]string) {
			reconciledAnnotations := internalmetadata.ReconcileAndFilterAnnotations(existingAnnotations, defaultAnnotations...)
			Expect(reconciledAnnotations).To(Equal(expectedAnnotations))
		},

		Entry("existingAnnotations is nil return merged and filtered default annotations",
			map[string]string{
				"foo":                "bar",
				"another-annotation": "some-value",
			}, nil, defaultOne, defaultAnnotationsWithK8s),

		Entry("no default annotations are present returns existing annotations",
			map[string]string{
				"existingAnnotation": "value",
				"k8s.io.annotation":  "annot",
			},
			map[string]string{
				"existingAnnotation": "value",
				"k8s.io.annotation":  "annot",
			}),

		Entry("filter default annotations and merge with existing annotations",
			map[string]string{
				"existingAnnotation": "value",
				"k8s.io.annotation":  "annot",
				"foo":                "bar",
				"another-annotation": "some-value",
			},
			map[string]string{
				"existingAnnotation": "value",
				"k8s.io.annotation":  "annot",
			}, defaultOne, defaultAnnotationsWithK8s),
	)
})
