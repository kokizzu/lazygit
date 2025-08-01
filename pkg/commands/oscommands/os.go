package oscommands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-errors/errors"
	"github.com/samber/lo"

	"github.com/atotto/clipboard"
	"github.com/jesseduffield/lazygit/pkg/common"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// OSCommand holds all the os commands
type OSCommand struct {
	*common.Common
	Platform *Platform
	getenvFn func(string) string
	guiIO    *guiIO

	removeFileFn func(string) error

	Cmd *CmdObjBuilder

	tempDir string
}

// Platform stores the os state
type Platform struct {
	OS                          string
	Shell                       string
	ShellArg                    string
	PrefixForShellFunctionsFile string
	OpenCommand                 string
	OpenLinkCommand             string
}

// NewOSCommand os command runner
func NewOSCommand(common *common.Common, config config.AppConfigurer, platform *Platform, guiIO *guiIO) *OSCommand {
	c := &OSCommand{
		Common:       common,
		Platform:     platform,
		getenvFn:     os.Getenv,
		removeFileFn: os.RemoveAll,
		guiIO:        guiIO,
		tempDir:      config.GetTempDir(),
	}

	runner := &cmdObjRunner{log: common.Log, guiIO: guiIO}
	c.Cmd = &CmdObjBuilder{runner: runner, platform: platform}

	return c
}

func (c *OSCommand) LogCommand(cmdStr string, commandLine bool) {
	c.Log.WithField("command", cmdStr).Info("RunCommand")

	c.guiIO.logCommandFn(cmdStr, commandLine)
}

// FileType tells us if the file is a file, directory or other
func FileType(path string) string {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "other"
	}
	if fileInfo.IsDir() {
		return "directory"
	}
	return "file"
}

func (c *OSCommand) OpenFile(filename string) error {
	commandTemplate := c.UserConfig().OS.Open
	if commandTemplate == "" {
		commandTemplate = config.GetPlatformDefaultConfig().Open
	}
	templateValues := map[string]string{
		"filename": c.Quote(filename),
	}
	command := utils.ResolvePlaceholderString(commandTemplate, templateValues)
	return c.Cmd.NewShell(command, c.UserConfig().OS.ShellFunctionsFile).Run()
}

func (c *OSCommand) OpenLink(link string) error {
	commandTemplate := c.UserConfig().OS.OpenLink
	if commandTemplate == "" {
		commandTemplate = config.GetPlatformDefaultConfig().OpenLink
	}
	templateValues := map[string]string{
		"link": c.Quote(link),
	}

	command := utils.ResolvePlaceholderString(commandTemplate, templateValues)
	return c.Cmd.NewShell(command, c.UserConfig().OS.ShellFunctionsFile).Run()
}

// Quote wraps a message in platform-specific quotation marks
func (c *OSCommand) Quote(message string) string {
	return c.Cmd.Quote(message)
}

// AppendLineToFile adds a new line in file
func (c *OSCommand) AppendLineToFile(filename, line string) error {
	msg := utils.ResolvePlaceholderString(
		c.Tr.Log.AppendingLineToFile,
		map[string]string{
			"line":     line,
			"filename": filename,
		},
	)
	c.LogCommand(msg, false)

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return utils.WrapError(err)
	}
	defer f.Close()

	info, err := os.Stat(filename)
	if err != nil {
		return utils.WrapError(err)
	}

	if info.Size() > 0 {
		// read last char
		buf := make([]byte, 1)
		if _, err := f.ReadAt(buf, info.Size()-1); err != nil {
			return utils.WrapError(err)
		}

		// if the last byte of the file is not a newline, add it
		if []byte("\n")[0] != buf[0] {
			_, err = f.WriteString("\n")
		}
	}

	if err == nil {
		_, err = f.WriteString(line + "\n")
	}

	if err != nil {
		return utils.WrapError(err)
	}
	return nil
}

// CreateFileWithContent creates a file with the given content
func (c *OSCommand) CreateFileWithContent(path string, content string) error {
	msg := utils.ResolvePlaceholderString(
		c.Tr.Log.CreateFileWithContent,
		map[string]string{
			"path": path,
		},
	)
	c.LogCommand(msg, false)
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		c.Log.Error(err)
		return err
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		c.Log.Error(err)
		return utils.WrapError(err)
	}

	return nil
}

// Remove removes a file or directory at the specified path
func (c *OSCommand) Remove(filename string) error {
	msg := utils.ResolvePlaceholderString(
		c.Tr.Log.Remove,
		map[string]string{
			"filename": filename,
		},
	)
	c.LogCommand(msg, false)
	err := os.RemoveAll(filename)
	return utils.WrapError(err)
}

// FileExists checks whether a file exists at the specified path
func (c *OSCommand) FileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// PipeCommands runs a heap of commands and pipes their inputs/outputs together like A | B | C
func (c *OSCommand) PipeCommands(cmdObjs ...*CmdObj) error {
	cmds := lo.Map(cmdObjs, func(cmdObj *CmdObj, _ int) *exec.Cmd {
		return cmdObj.GetCmd()
	})

	logCmdStr := strings.Join(
		lo.Map(cmdObjs, func(cmdObj *CmdObj, _ int) string {
			return cmdObj.ToString()
		}),
		" | ",
	)

	c.LogCommand(logCmdStr, true)

	for i := range len(cmds) - 1 {
		stdout, err := cmds[i].StdoutPipe()
		if err != nil {
			return err
		}

		cmds[i+1].Stdin = stdout
	}

	// keeping this here in case I adapt this code for some other purpose in the future
	// cmds[len(cmds)-1].Stdout = os.Stdout

	finalErrors := []string{}

	wg := sync.WaitGroup{}
	wg.Add(len(cmds))

	for _, cmd := range cmds {
		go utils.Safe(func() {
			stderr, err := cmd.StderrPipe()
			if err != nil {
				c.Log.Error(err)
			}

			if err := cmd.Start(); err != nil {
				c.Log.Error(err)
			}

			if b, err := io.ReadAll(stderr); err == nil {
				if len(b) > 0 {
					finalErrors = append(finalErrors, string(b))
				}
			}

			if err := cmd.Wait(); err != nil {
				c.Log.Error(err)
			}

			wg.Done()
		})
	}

	wg.Wait()

	if len(finalErrors) > 0 {
		return errors.New(strings.Join(finalErrors, "\n"))
	}
	return nil
}

func (c *OSCommand) CopyToClipboard(str string) error {
	escaped := strings.ReplaceAll(str, "\n", "\\n")
	truncated := utils.TruncateWithEllipsis(escaped, 40)

	msg := utils.ResolvePlaceholderString(
		c.Tr.Log.CopyToClipboard,
		map[string]string{
			"str": truncated,
		},
	)
	c.LogCommand(msg, false)
	if c.UserConfig().OS.CopyToClipboardCmd != "" {
		cmdStr := utils.ResolvePlaceholderString(c.UserConfig().OS.CopyToClipboardCmd, map[string]string{
			"text": c.Cmd.Quote(str),
		})
		return c.Cmd.NewShell(cmdStr, c.UserConfig().OS.ShellFunctionsFile).Run()
	}

	return clipboard.WriteAll(str)
}

func (c *OSCommand) PasteFromClipboard() (string, error) {
	var s string
	var err error
	if c.UserConfig().OS.CopyToClipboardCmd != "" {
		cmdStr := c.UserConfig().OS.ReadFromClipboardCmd
		s, err = c.Cmd.NewShell(cmdStr, c.UserConfig().OS.ShellFunctionsFile).RunWithOutput()
	} else {
		s, err = clipboard.ReadAll()
	}

	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(s, "\r\n", "\n"), nil
}

func (c *OSCommand) RemoveFile(path string) error {
	msg := utils.ResolvePlaceholderString(
		c.Tr.Log.RemoveFile,
		map[string]string{
			"path": path,
		},
	)
	c.LogCommand(msg, false)

	return c.removeFileFn(path)
}

func (c *OSCommand) Getenv(key string) string {
	return c.getenvFn(key)
}

func (c *OSCommand) GetTempDir() string {
	return c.tempDir
}

// GetLazygitPath returns the path of the currently executed file
func GetLazygitPath() string {
	ex, err := os.Executable() // get the executable path for git to use
	if err != nil {
		ex = os.Args[0] // fallback to the first call argument if needed
	}
	return `"` + filepath.ToSlash(ex) + `"`
}

func (c *OSCommand) UpdateWindowTitle() error {
	if c.Platform.OS != "windows" {
		return nil
	}
	path, getWdErr := os.Getwd()
	if getWdErr != nil {
		return getWdErr
	}
	argString := fmt.Sprint("title ", filepath.Base(path), " - Lazygit")
	return c.Cmd.NewShell(argString, c.UserConfig().OS.ShellFunctionsFile).Run()
}
