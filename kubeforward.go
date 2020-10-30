package main

import (
	"bytes"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type portForward struct {
	name     string
	podPort  string
	hostPort string
}

// Gets pod name from the first pod on deployment array
// Returns pod name and error output
func getPodName(deploy string) (string, error) {

	cmd := exec.Command("kubectl", "get", "pods", "-l", fmt.Sprintf("app=%s", deploy), "-o", "jsonpath={.items[0].metadata.name}")
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	err := cmd.Run()

	if err != nil {
		cmd.Wait()
		return string(cmdOutput.Bytes()), fmt.Errorf("%s pod not found", deploy)
	}

	cmd.Wait()
	return string(cmdOutput.Bytes()), nil
}

// Start port-forward in a goroutine
func startForward(deploy, hostPort, podPort string, wg *sync.WaitGroup) {

	// Finish goroutine at the end of this function
	defer wg.Done()

	for {
		podName, err := getPodName(deploy)

		if err != nil {
			// If pod name wasn't found, it breaks the loop and finishes goroutine
			fmt.Println(err)
			break
		}

		t := time.Now().Format("2006-01-02 15:04:05")
		cmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("pods/%s", podName), fmt.Sprintf("%s:%s", hostPort, podPort))

		//Execution modes (verbose, debug, standard)
		if isFlagPassed("verbose") {
			var stdBuffer bytes.Buffer
			mw := io.MultiWriter(os.Stdout, &stdBuffer)

			cmd.Stdout = mw
			cmd.Stderr = mw
			fmt.Printf("[%s] Forwarding %-12s port %3s to local port %4s [pod: %s]\n", t, deploy, podPort, hostPort, podName)

			// Execute the command
			if err := cmd.Run(); err != nil {
				log.Panic(err)
			}

			log.Println(stdBuffer.String())

			cmd.Wait()

			t = time.Now().Format("2006-01-02 15:04:05")
			fmt.Printf("[%s] %s port-forward failed. Retrying...\n", t, strings.ToUpper(deploy))
		} else if isFlagPassed("quiet") {
			cmdOutput := &bytes.Buffer{}
			cmd.Stdout = cmdOutput
			err = cmd.Run()

			if err != nil {
				os.Stderr.WriteString(err.Error())
				fmt.Println("")
			}
		} else {
			cmdOutput := &bytes.Buffer{}
			cmd.Stdout = cmdOutput
			fmt.Printf("[%s] Forwarding %-12s port %3s to local port %4s [pod: %s]\n", t, deploy, podPort, hostPort, podName)
			err = cmd.Run()

			if err != nil {
				os.Stderr.WriteString(err.Error())
				cmd.Wait()

				t = time.Now().Format("2006-01-02 15:04:05")
				fmt.Printf("[%s] %s port-forward failed. Retrying...\n", t, strings.ToUpper(deploy))
				fmt.Println("")
			}
			cmd.Wait()

			t = time.Now().Format("2006-01-02 15:04:05")
			fmt.Printf("[%s] %s port-forward failed. Retrying...\n", t, strings.ToUpper(deploy))
		}
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)

	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

type Yaml struct {
	Deployment []Deployment
}

type Deployment struct {
	Name     string
	Hostport string
	Podport  string
}

func getConfFile(filename string) Yaml {
	data, _ := ioutil.ReadFile(filename)
	config := Yaml{}

	err := yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	return config

}

// Return config file path if exists and an array of deploy configurations
func argInfo() (string, []string) {
	var fvar string
	var svar bool = false

	flag.StringVar(&fvar, "file", "", "string as path")
	flag.BoolVar(&svar, "quiet", true, "silent mode enable")
	flag.BoolVar(&svar, "verbose", true, "debug mode enable")
	flag.Parse()

	var path string
	if fvar != "" {
		path, _ = filepath.Abs(fvar)
	} else {
		path = "deploy.yaml"
	}

	return path, flag.Args()

}

// Get arguments and appends them to deployments array
func getArgsConfig(config *Yaml, a []string) {

	for _, arg := range a {
		var new_dep Deployment
		var flag bool

		// If parameter is not correct, its config won't be added
		if !ValidDeployInfo(arg) {
			fmt.Println("Invalid deployment info format: ", string(arg), " ignored")
			continue
		}

		for i, dp := range config.Deployment {

			fields := strings.Split(arg, ":")
			new_dep.Name = fields[0]
			new_dep.Hostport = string(fields[1])
			new_dep.Podport = string(fields[2])

			// If deployment is already in config, overwrite it
			if new_dep.Name == dp.Name {
				config.Deployment[i] = new_dep
				flag = true
				continue
			}
		}

		if !flag {
			config.Deployment = append(config.Deployment, new_dep)
		}

	} //for

}

func ValidDeployInfo(s string) bool {
	s = strings.ToLower(s)
	valid := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_\.]{1,250}[:][0-9]{1,5}[:][0-9]{1,5}$`)
	// valid := regexp.MustCompile(`^[a-z0-9]$`)
	return valid.MatchString(s)
}

func showHelp() {
	fmt.Println("kubeforward: missing either argument or deploy.yaml file.")
	fmt.Println("Use: kubeforward <deploy_name>:<host_port>:<pod_port> [<deploy_name>:<host_port>:<pod_port> ...]")
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func main() {

	filename, args := argInfo()
	var config Yaml

	// Check whether either any parameter or config file was received
	if fileExists(filename) {
		config = getConfFile(filename)
	} else {
		if len(args) == 0 {
			showHelp()
			os.Exit(2)
		}
	}

	getArgsConfig(&config, args)

	var wg sync.WaitGroup
	wg.Add(len(config.Deployment))

	for _, dp := range config.Deployment {
		// Initiates a goroutine for every port-forward
		go startForward(dp.Name, dp.Hostport, dp.Podport, &wg)
	}

	wg.Wait()
}
