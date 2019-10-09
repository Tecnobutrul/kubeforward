package main

import (
	"bytes"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

	cmd := exec.Command("kubectl", "get", "pods", "--namespace", "default", "-l", fmt.Sprintf("app=%s", deploy), "-o", "jsonpath={.items[0].metadata.name}")
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
		fmt.Printf("[%s] Forwarding %-12s port %3s to local port %4s [pod: %s]\n", t, deploy, podPort, hostPort, podName)
		cmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("pods/%s", podName), fmt.Sprintf("%s:%s", hostPort, podPort))
		cmdOutput := &bytes.Buffer{}
		cmd.Stdout = cmdOutput
		err = cmd.Run()

		if err != nil {
			os.Stderr.WriteString(err.Error())
			fmt.Println("")
		}

		cmd.Wait()

		t = time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("[%s] %s port-forward failed. Retrying...\n", t, strings.ToUpper(deploy))
	}
}

func file_exists(filename string) bool {
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

func get_conf_file(filename string) Yaml {
	data, _ := ioutil.ReadFile(filename)
	config := Yaml{}

	err := yaml.Unmarshal([]byte(data), &config)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	return config

}

func arg_info() (string, []string) {
	var fvar string

	flag.StringVar(&fvar, "file", "foo", "a string var")
	flag.Parse()

	var path string
	if fvar != "foo" {
		path, _ = filepath.Abs(fvar)
	} else {
		path = "deploy.yaml"
	}

	return path, flag.Args()

}

// Get arguments and appends them to deployments array
func get_args(config *Yaml, a []string) {

	for _, arg := range a {
		var new_dep Deployment

		fields := strings.Split(arg, ":")
		new_dep.Name = fields[0]
		new_dep.Hostport = string(fields[1])
		new_dep.Podport = string(fields[2])
		config.Deployment = append(config.Deployment, new_dep)
	}

}

// func ValidDeployInfo(s string) bool {
// s = strings.ToLower(s)
// var reSlash = regexp.MustCompile(`^$)`)
// return reSlash.MatchString(s)
// }

func show_help() {
	fmt.Println("kubeforward: missing either argument or deploy.yaml file.")
	fmt.Println("Use: kubeforward <deploy_name>:<host_port>:<pod_port> [<deploy_name>:<host_port>:<pod_port> ...]")
}

func main() {

	filename, args := arg_info()
	var config Yaml

	// Check whether either any parameter or config file was received
	if file_exists(filename) {
		config = get_conf_file(filename)
	} else {
		if len(args) == 0 {
			show_help()
			os.Exit(2)
		}
	}

	get_args(&config, args)

	var wg sync.WaitGroup
	wg.Add(len(config.Deployment))

	for _, dp := range config.Deployment {
		// Initiates a goroutine for every port-forward
		go startForward(dp.Name, dp.Hostport, dp.Podport, &wg)
	}

	wg.Wait()
}
