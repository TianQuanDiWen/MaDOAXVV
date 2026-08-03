package agentserver

import (
	"fmt"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// Registry 保存项目提供给 MaaFramework 的自定义识别和自定义动作。
// 统一注册可以避免各业务模块直接操作全局 AgentServer 状态。
type Registry struct {
	recognitions map[string]maa.CustomRecognitionRunner
	actions      map[string]maa.CustomActionRunner
}

// NewRegistry 创建一个空的扩展注册表。
func NewRegistry() *Registry {
	return &Registry{
		recognitions: make(map[string]maa.CustomRecognitionRunner),
		actions:      make(map[string]maa.CustomActionRunner),
	}
}

// AddRecognition 添加一个 MaaFramework 自定义识别实现。
func (r *Registry) AddRecognition(name string, runner maa.CustomRecognitionRunner) error {
	if name == "" || runner == nil {
		return fmt.Errorf("custom recognition requires a name and runner")
	}
	if _, exists := r.recognitions[name]; exists {
		return fmt.Errorf("custom recognition %q is already registered", name)
	}
	r.recognitions[name] = runner
	return nil
}

// AddAction 添加一个 MaaFramework 自定义动作实现。
func (r *Registry) AddAction(name string, runner maa.CustomActionRunner) error {
	if name == "" || runner == nil {
		return fmt.Errorf("custom action requires a name and runner")
	}
	if _, exists := r.actions[name]; exists {
		return fmt.Errorf("custom action %q is already registered", name)
	}
	r.actions[name] = runner
	return nil
}

// RegisterAgentServer 将当前注册表一次性挂载到 MaaFramework AgentServer。
func (r *Registry) RegisterAgentServer() error {
	for name, runner := range r.recognitions {
		if err := maa.AgentServerRegisterCustomRecognition(name, runner); err != nil {
			return fmt.Errorf("register custom recognition %q: %w", name, err)
		}
	}
	for name, runner := range r.actions {
		if err := maa.AgentServerRegisterCustomAction(name, runner); err != nil {
			return fmt.Errorf("register custom action %q: %w", name, err)
		}
	}
	return nil
}

// BuildRegistry 是项目自定义能力的统一装配入口。
// 后续的分数识别、场景判断和特殊操作应在这里按模块注册。
func BuildRegistry() (*Registry, error) {
	registry := NewRegistry()
	return registry, nil
}

