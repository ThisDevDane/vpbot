package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var (
	odinPath      string
	mainRegex     *regexp.Regexp
	osImportRegex *regexp.Regexp
)

func initOdin() {
	odinPath, _ = exec.LookPath("odin")
	mainRegex = regexp.MustCompile(mainRegexStr)
}

func odinRunHandle(session *discordgo.Session, msg *discordgo.MessageCreate) {
	mesg := strings.TrimPrefix(msg.Content, "!odinrun")
	mesg = strings.TrimSpace(mesg)

	i1 := strings.Index(mesg, "```")
	if i1 < 0 {
		session.ChannelMessageSend(msg.ChannelID, "Please put your code in a code block")
		return
	}
	offset := i1 + 3
	i2 := strings.Index(mesg[offset:], "```")
	if i2 < 0 {
		session.ChannelMessageSend(msg.ChannelID, "Incomplete code block")
		return
	}

	code := mesg[offset : i2+offset]
	mainInCode := mainRegex.MatchString(code)

	if mainInCode == false {
		code = strings.ReplaceAll(odinProgramTemplate, "REPLACE_ME", code)
	}

	session.ChannelMessageSend(msg.ChannelID, "Running code...")

	f, err := os.Create("test.odin")
	defer os.Remove("test.odin")

	if err != nil {
		session.ChannelMessageSend(msg.ChannelID, "Couldn't create file to run!!")
		return
	}

	w := bufio.NewWriter(f)
	w.WriteString(code)
	w.Flush()
	f.Close()

	var out bytes.Buffer
	cmd := exec.Cmd{
		Path:   odinPath,
		Args:   []string{"odin", "run", "test.odin"},
		Stdout: &out,
		Stderr: &out,
	}

	cmd.Run()

	resp := fmt.Sprintf("Output: ```\n%v\n```", out.String())

	session.ChannelMessageSend(msg.ChannelID, resp)
}

const mainRegexStr = `main\s::\sproc\(\)\s{(?:(?:.|\n)*)}`

const odinProgramTemplate = `
package main

import "core:fmt"
import "core:math"
import "core:math/linalg"
import "core:mem"
import "core:strings"

main :: proc() {
	REPLACE_ME;
}
`
