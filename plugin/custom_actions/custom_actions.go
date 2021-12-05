package customact

import (
	"fmt"
	"github.com/blinkops/blink-openapi-sdk/consts"
	"github.com/blinkops/blink-openapi-sdk/zip"
	"github.com/blinkops/blink-sdk/plugin"
	"github.com/blinkops/blink-sdk/plugin/actions"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	if os.Getenv(consts.ENVStatusKey) != "" {
		unzipCustomActions(currentDirectory + c.ActionsFolderPath)
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

func unzipCustomActions(rootPath string) {
	log.Info("Prod environment - trying to unzip zipped custom actions")
	err := filepath.WalkDir(rootPath, walkDirFunc)
	if err != nil {
		log.Panic("Failed to unzip custom actions", err)
	}
}

func walkDirFunc(filePath string, _ fs.DirEntry, err error) error {
	if err != nil || !strings.HasSuffix(filePath, ".gz") {
		return nil
	}
	err = zip.UnzipFile(strings.TrimSuffix(filePath, ".gz"))
	if err != nil {
		log.Panic("Failed to unzip custom actions", err)
		return err
	}
	return nil
}

