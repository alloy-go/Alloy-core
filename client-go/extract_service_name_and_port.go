package services

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type ServiceManifest struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Ports []struct {
			Port       int         `yaml:"port"`
			TargetPort interface{} `yaml:"targetPort"` // can be int or string
		} `yaml:"ports"`
	} `yaml:"spec"`
}

func ExtractServiceNameAndPort(serviceYAML []byte) (string, int, error) {
	var svc ServiceManifest
	if err := yaml.Unmarshal(serviceYAML, &svc); err != nil {
		return "", 0, fmt.Errorf("service yaml unmarshal: %w", err)
	}
	if svc.Metadata.Name == "" {
		return "", 0, fmt.Errorf("service name missing in metadata.name")
	}
	if len(svc.Spec.Ports) == 0 {
		return "", 0, fmt.Errorf("service spec.ports is empty")
	}
	if svc.Spec.Ports[0].Port == 0 {
		return "", 0, fmt.Errorf("service spec.ports[0].port missing/0")
	}
	return svc.Metadata.Name, svc.Spec.Ports[0].Port, nil
}
