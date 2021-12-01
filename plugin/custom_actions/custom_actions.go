package customact

import (
	"fmt"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/actions"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
)

type ActionHandler func(*plugin.ActionContext, *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error)

type CustomActions struct {
	Actions           map[string]ActionHandler
	ActionsFolderPath string
}

func (c CustomActions) GetActions() []plugin.Action {
	currentDirectory, err := os.Getwd()
	if err != nil {
		log.Error("could not get the current directory")
		return []plugin.Action{}
	}
	actionsFromDisk, err := actions.LoadActionsFromDisk(path.Join(currentDirectory, c.ActionsFolderPath))
	if err != nil {
		log.Error("failed to load Actions from disk")
		return []plugin.Action{}
	}
	return actionsFromDisk

}

func (c CustomActions) HasAction(actionName string) bool {
	if c.Actions == nil {
		return false
	}
	if _, ok := c.Actions[actionName]; !ok {
		return false
	}
	return true
}

func (c CustomActions) Execute(actionContext *plugin.ActionContext, request *plugin.ExecuteActionRequest) (*plugin.ExecuteActionResponse, error) {
	if _, ok := c.Actions[request.Name]; !ok {
		return nil, fmt.Errorf("custom action not found")
	}

	return c.Actions[request.Name](actionContext, request)

}
