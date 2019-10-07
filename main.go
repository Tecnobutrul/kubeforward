package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
			// If pod name wasn't found break the loop and finishes goroutine
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

func main() {

	var podList [6]portForward
	podList[0] = portForward{name: "auth", podPort: "80", hostPort: "8080"}
	podList[1] = portForward{name: "calibration", podPort: "80", hostPort: "8081"}
	podList[2] = portForward{name: "realtime", podPort: "80", hostPort: "8082"}
	podList[3] = portForward{name: "profiles", podPort: "80", hostPort: "8083"}
	podList[4] = portForward{name: "alarms", podPort: "80", hostPort: "8084"}
	podList[5] = portForward{name: "gateway", podPort: "80", hostPort: "8085"}

	var wg sync.WaitGroup
	wg.Add(len(podList))

	for idx, _ := range podList {
		// Initiates a goroutine for every port-forward
		go startForward(podList[idx].name, podList[idx].hostPort, podList[idx].podPort, &wg)
	}
	wg.Wait()
}
