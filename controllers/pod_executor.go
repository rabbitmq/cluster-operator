package controllers

import (
	"bufio"
	"bytes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type PodExecutor interface {
	Exec(clientset *kubernetes.Clientset, clusterConfig *rest.Config, namespace, podName, containerName string, command ...string) (string, string, error)
}

func NewPodExecutor() PodExecutor { return &rabbitmqPodExecutor{} }

type rabbitmqPodExecutor struct{}

func (p *rabbitmqPodExecutor) Exec(clientset *kubernetes.Clientset, clusterConfig *rest.Config, namespace, podName, containerName string, command ...string) (string, string, error) {
	request := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
			Stdin:     false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clusterConfig, "POST", request.URL())
	if err != nil {
		return "", "", err
	}

	stdOut := bytes.Buffer{}
	stdErr := bytes.Buffer{}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: bufio.NewWriter(&stdOut),
		Stderr: bufio.NewWriter(&stdErr),
		Stdin:  nil,
		Tty:    false,
	})

	return stdOut.String(), stdErr.String(), err
}
