package main

import (
	"fmt"
	"github.com/df-mc/dragonfly/dragonfly"
	"github.com/df-mc/dragonfly/dragonfly/player/chat"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"github.com/t14raptor/df-bds-gen"
	"io/ioutil"
	"os"
	"os/exec"
	"time"
)

func main() {
	log := logrus.New()
	log.Formatter = &logrus.TextFormatter{ForceColors: true}
	log.Level = logrus.DebugLevel

	chat.Global.Subscribe(chat.StdoutSubscriber{})

	config, err := readConfig()
	if err != nil {
		panic(err)
	}

	server := dragonfly.New(&config.Config, log)
	server.CloseOnProgramEnd()
	if err := server.Start(); err != nil {
		panic(err)
	}

	log.Debugln("Attempting to start BDS")

	cmd := exec.Command(config.Generator.BdsPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 10)

	c := gen.NewClient(log)

	if err = c.StartClient(config.Generator.BdsAddress, config.Generator.ClientChunkRadius); err != nil {
		panic(err)
	}

	server.World().Generator(gen.NewGenerator(c))
	for {
		p, err := server.Accept()
		if err != nil {
			return
		}

		_ = p
	}
}

type config struct {
	dragonfly.Config
	Generator struct {
		BdsAddress        string
		BdsPath           string
		ClientChunkRadius int
	}
}

func readConfig() (config, error) {
	c := config{}
	if _, err := os.Stat("config.toml"); os.IsNotExist(err) {
		data, err := toml.Marshal(c)
		if err != nil {
			return c, fmt.Errorf("failed encoding default config: %v", err)
		}
		if err := ioutil.WriteFile("config.toml", data, 0644); err != nil {
			return c, fmt.Errorf("failed creating config: %v", err)
		}
		return c, nil
	}
	data, err := ioutil.ReadFile("config.toml")
	if err != nil {
		return c, fmt.Errorf("error reading config: %v", err)
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("error decoding config: %v", err)
	}
	return c, nil
}
