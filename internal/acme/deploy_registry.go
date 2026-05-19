package acme

import (
	"errors"
	"fmt"
	"strings"
)

type DeployRegistry struct {
	drivers map[string]DeployDriver
}

func NewDeployRegistry(drivers ...DeployDriver) *DeployRegistry {
	r := &DeployRegistry{drivers: map[string]DeployDriver{}}
	for _, d := range drivers {
		if d == nil {
			continue
		}
		r.drivers[d.Kind()] = d
	}
	return r
}

func (r *DeployRegistry) Get(kind string) (DeployDriver, error) {
	if r == nil {
		return nil, errors.New("部署 driver registry 未初始化")
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	d, ok := r.drivers[kind]
	if !ok {
		return nil, fmt.Errorf("不支持的部署类型：%s", kind)
	}
	return d, nil
}
