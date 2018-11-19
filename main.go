package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	semver "github.com/hashicorp/go-version"
)

// Config model
type Config struct {
	Workdir    string `env:"workdir"`
	Command    string `env:"command,required"`
	NpmVersion string `env:"npm_version"`
}

func getNpmVersionFromPackageJSON() (string, error) {
	jsonStr, err := fileutil.ReadStringFromFile("package.json")
	if err != nil {
		return "", fmt.Errorf("package.json file read error: %s", err)
	}

	var ver string
	if ver, err = extractNpmVersion(jsonStr); err != nil {
		return "", fmt.Errorf("package json content error: %s", err)
	}
	return ver, nil
}

func extractNpmVersion(jsonStr string) (string, error) {
	type jsonModel struct {
		Engines struct {
			Npm string
		}
	}

	var m jsonModel
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return "", fmt.Errorf("json unmarshal error: %s", err)
	}

	if m.Engines.Npm == "" {
		return "", nil
	}

	v, err := semver.NewVersion(m.Engines.Npm)
	if err != nil {
		return "", fmt.Errorf("engines.npm is not valid semver string")
	}

	return v.String(), nil
}

func createInstallNpmCommand(os string) *command.Model {
	var args []string
	switch os {
	case "darwin":
		args = []string{"brew", "install", "node"}
	case "linux":
		args = []string{"apt-get", "-y", "install", "npm"}
	}

	return command.New(args[0], args[1:]...)
}

func installLatestNpm() (string, error) {
	cmd := createInstallNpmCommand(runtime.GOOS)

	var out string
	out, err := command.RunCmdAndReturnTrimmedOutput(cmd.GetCmd())

	if err != nil {
		return out, fmt.Errorf("error running npm install: %s", err)
	}

	return out, nil
}

func runAndLog(cmd *command.Model) (string, error) {
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return out, fmt.Errorf("error running npm command: %s", err)
	}

	log.Donef(fmt.Sprintf("$ %s", cmd.PrintableCommandArgs()))
	log.Printf(out)

	return out, nil
}

func setNpmVersion(ver string) (string, error) {
	cmd := command.New("npm", "install", "-g", fmt.Sprintf("npm@%s", ver))
	out, err := runAndLog(cmd)
	if err != nil {
		return out, fmt.Errorf("error running npm install: %s", err)
	}

	return out, nil
}

func failf(f string, args ...interface{}) {
	log.Errorf(f, args...)
	fmt.Println()
	os.Exit(1)
}

func main() {
	var config Config
	if err := stepconf.Parse(&config); err != nil {
		failf("error parsing step config: %s", err)
	}
	stepconf.Print(config)

	if config.NpmVersion == "" {
		fmt.Println()
		log.Infof("Autodetecting npm version")

		log.Printf("Checking package.json for npm version")
		ver, err := getNpmVersionFromPackageJSON()
		if err != nil {
			failf("error processing package.json: %s", err)
		}

		if ver == "" {
			log.Warnf("No npm version found in package.json")

			log.Printf("Locating system installed npm")
			path, err := exec.LookPath("npm")
			if err != nil {
				log.Warnf("npm not found on PATH")

				log.Printf("Installing latest npm")
				ver = "latest"

				out, err := runAndLog(createInstallNpmCommand(runtime.GOOS))
				if err != nil {
					log.Errorf(out)
					failf("Error installing npm: %s", err)
				}

				log.Printf(out)
			} else {
				log.Printf("npm found at %s", path)

				cmd := command.New("npm", "--version")
				out, err := runAndLog(cmd)
				if err != nil {
					log.Warnf("Error getting installed npm version: %s", err)
				}
				ver = out
			}
		} else {
			fmt.Println(ver)
			fmt.Println()
			log.Infof("Setting npm version to %s", ver)

			out, err := setNpmVersion(ver)
			if err != nil {
				log.Errorf(out)
				failf("Error setting npm version to %s: %s", ver, err)
			}
		}
	} else {
		fmt.Println()
		log.Infof("Setting npm version to %s", config.NpmVersion)

		out, err := setNpmVersion(config.NpmVersion)
		if err != nil {
			log.Errorf(out)
			failf("Error setting npm version to %s: %s", config.NpmVersion, err)
		}
	}

	fmt.Println()
	log.Infof("Running user provided command")

	cmd := command.New("npm", config.Command)
	out, err := runAndLog(cmd)
	if err != nil {
		log.Errorf(out)
		failf("Error running command %s: %s", config.Command, err)
	}

	fmt.Println()
	log.Successf("Step success")
}
