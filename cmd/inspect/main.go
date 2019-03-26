package main

import (
	"flag"
	"fmt"
	log "github.com/astaxie/beego/logs"
	"os"

	"k8s.io/api/core/v1"
)

const (
	resourceName         = "shared-gpu/gpu-mem"
	countName            = "shared-gpu/gpu-count"
	gpuCountKey          = "shared-gpu/nvidia_count"
	cardNameKey          = "shared-gpu/nvidia_name"
	gpuMemKey            = "shared-gpu/nvidia_mem"
	pluginComponentKey   = "component"
	pluginComponentValue = "gpushare-device-plugin"

	envNVGPUID        = "SHARED_GPU_MEM_IDX"
	envPodGPUMemory   = "SHARED_GPU_MEM_POD"
	envTOTALGPUMEMORY = "SHARED_GPU_MEM_DEV"
	logPath           = "/var/log/device-plugin"
)

func beegoInit() {
	log.EnableFuncCallDepth(true)
	log.SetLogFuncCallDepth(3)
	if !pathExists(logPath) {
		fmt.Printf("dir: %s not found.", logPath)
		err := os.MkdirAll(logPath, 0711)
		if err != nil {
			fmt.Printf("mkdir %s failed: %v", logPath, err)
		}
	}
	err := log.SetLogger(log.AdapterMultiFile, `{"filename":"/var/log/device-plugin/nvidia.log","separate":["emergency", "alert", 
			"critical", "error", "warning", "notice", "info", "debug"]}`)
	if err != nil {
		fmt.Println(err)
	}
	err = log.SetLogger(log.AdapterConsole, `{"level":6}`)
	if err != nil {
		fmt.Println(err)
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func init() {
	beegoInit()
	kubeInit()
	// checkpointInit()
}

func main() {
	var nodeName string
	// nodeName := flag.String("nodeName", "", "nodeName")
	details := flag.Bool("d", false, "details")
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 {
		nodeName = args[0]
	}

	var pods []v1.Pod
	var nodes []v1.Node
	var err error

	if nodeName == "" {
		nodes, err = getAllSharedGPUNode()
		if err == nil {
			pods, err = getActivePodsInAllNodes()
		}
	} else {
		nodes, err = getNodes(nodeName)
		if err == nil {
			pods, err = getActivePodsByNode(nodeName)
		}
	}

	if err != nil {
		fmt.Printf("Failed due to %v", err)
		os.Exit(1)
	}

	nodeInfos, err := buildAllNodeInfos(pods, nodes)
	if err != nil {
		fmt.Printf("Failed due to %v", err)
		os.Exit(1)
	}
	if *details {
		displayDetails(nodeInfos)
	} else {
		displaySummary(nodeInfos)
	}

}
