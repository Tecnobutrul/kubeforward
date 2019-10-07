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

func getPodName(deploy string) string {
	cmd := exec.Command("kubectl", "get", "pods", "--namespace", "default", "-l", fmt.Sprintf("app=%s", deploy), "-o", "jsonpath={.items[0].metadata.name}")
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	err := cmd.Run()
	if err != nil {
		os.Stderr.WriteString(err.Error())
	}
	cmd.Wait()
	return string(cmdOutput.Bytes())
}

func startForward(deploy, hostPort, podPort string, wg *sync.WaitGroup) {

	defer wg.Done()

	for {
		podName := getPodName(deploy)
		t := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("[%s] Forwarding %-12s port %3s to local port %4s [pod: %s]\n", t, deploy, podPort, hostPort, podName)
		// fmt.Println(deploy)
		// fmt.Println("================================================================")
		// fmt.Println(podName)
		cmd := exec.Command("kubectl", "port-forward", fmt.Sprintf("pods/%s", podName), fmt.Sprintf("%s:%s", hostPort, podPort))
		// fmt.Println("================================================================")
		// fmt.Println(cmd)
		cmdOutput := &bytes.Buffer{}
		cmd.Stdout = cmdOutput
		err := cmd.Run()
		// fmt.Println(err)

		if err != nil {
			os.Stderr.WriteString(err.Error())
			fmt.Println("")
		}

		// fmt.Print(string(cmdOutput.Bytes()))

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
		// fmt.Println(podList[idx])
		go startForward(podList[idx].name, podList[idx].hostPort, podList[idx].podPort, &wg)
	}
	wg.Wait()
}
