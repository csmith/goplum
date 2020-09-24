package exec

import (
	"context"
	"fmt"
	"github.com/csmith/goplum"
	"os/exec"
)

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	default:
		return nil
	}
}

func (p Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "command":
		return CommandCheck{}
	default:
		return nil
	}
}

type CommandCheck struct {
	Name string
	Arguments []string
}

func (c CommandCheck) Execute(ctx context.Context) goplum.Result {
	cmd := exec.CommandContext(ctx, c.Name, c.Arguments...)
	if err := cmd.Run(); err != nil {
		return goplum.FailingResult(fmt.Sprintf("command failed: %v", err))
	}
	return goplum.GoodResult()
}

func (c CommandCheck) Validate() error {
	if len(c.Name) == 0 {
		return fmt.Errorf("missing required argument: name")
	}
	return nil
}
