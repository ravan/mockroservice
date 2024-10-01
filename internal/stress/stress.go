package stress

import (
	"log/slog"
	"os/exec"
)

func Stress(args []string) {
	cmd := exec.Command("stress-ng", args...)
	err := cmd.Run()
	if err != nil {
		slog.Error("Error when running stress-ng", slog.Any("error", err))
	}
}
