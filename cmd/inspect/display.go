package main

import (
	"bytes"
	"fmt"
	"strconv"

	log "github.com/astaxie/beego/logs"
	"k8s.io/api/core/v1"
)

func displayDetails(nodeInfos []*NodeInfo) {
	//w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var (
		totalGPUMemInCluster int64
		usedGPUMemInCluster  int64
		prtLineLen           int
	)

	for _, nodeInfo := range nodeInfos {
		address := "unknown"
		if len(nodeInfo.node.Status.Addresses) > 0 {
			//address = nodeInfo.node.Status.Addresses[0].Address
			for _, addr := range nodeInfo.node.Status.Addresses {
				if addr.Type == v1.NodeExternalIP {
					address = addr.Address
					break
				}
			}
		}

		totalGPUMemInNode := nodeInfo.gpuTotalMemory
		if totalGPUMemInNode <= 0 {
			continue
		}

		log.Info("===================================================")
		log.Info("NAME:\t%s", nodeInfo.node.Name)
		log.Info("IPADDRESS:\t%s", address)
		log.Info("===================================================")

		usedGPUMemInNode := 0
		var buf bytes.Buffer
		buf.WriteString("NAME\tNAMESPACE\t")
		for i := 0; i < nodeInfo.gpuCount; i++ {
			buf.WriteString(fmt.Sprintf("GPU%d(Allocated)\t", i))
		}

		if nodeInfo.hasPendingGPUMemory() {
			buf.WriteString("Pending(Allocated)\t")
		}
		buf.WriteString("\n")
		log.Info(buf.String())

		var buffer bytes.Buffer
		for i, dev := range nodeInfo.devs {
			usedGPUMemInNode += dev.usedGPUMem
			for _, pod := range dev.pods {

				buffer.WriteString(fmt.Sprintf("%s\t%s\t", pod.Name, pod.Namespace))
				count := nodeInfo.gpuCount
				if nodeInfo.hasPendingGPUMemory() {
					count += 1
				}

				for k := 0; k < count; k++ {
					if k == i || (i == -1 && k == nodeInfo.gpuCount) {
						buffer.WriteString(fmt.Sprintf("%d\t", getGPUMemoryInPod(pod)))
					} else {
						buffer.WriteString("0\t")
					}
				}
				buffer.WriteString("\n")
			}
		}
		if prtLineLen == 0 {
			prtLineLen = buffer.Len() + 10
		}
		log.Info(buffer.String())

		var gpuUsageInNode float64 = 0
		if totalGPUMemInNode > 0 {
			gpuUsageInNode = float64(usedGPUMemInNode) / float64(totalGPUMemInNode) * 100
		} else {
			log.Info("-----------------------------")
		}

		log.Info("Allocated :\t%d (%d%%)", usedGPUMemInNode, int64(gpuUsageInNode))
		log.Info("Total :\t%d ", nodeInfo.gpuTotalMemory)
		// log.Info( "-----------------------------------------------------------------------------------------\n")
		var prtLine bytes.Buffer
		for i := 0; i < prtLineLen; i++ {
			prtLine.WriteString("-")
		}
		prtLine.WriteString("\n")
		log.Info(prtLine.String())
		totalGPUMemInCluster += int64(totalGPUMemInNode)
		usedGPUMemInCluster += int64(usedGPUMemInNode)
	}
	log.Info("")
	log.Info("")
	log.Info("Allocated/Total GPU Memory In Cluster:\t")
	log.Info("gpu: %s, allocated GPU Memory %s", strconv.FormatInt(totalGPUMemInCluster, 10),
		strconv.FormatInt(usedGPUMemInCluster, 10))

	var gpuUsage float64 = 0
	if totalGPUMemInCluster > 0 {
		gpuUsage = float64(usedGPUMemInCluster) / float64(totalGPUMemInCluster) * 100
	}
	log.Info("%s/%s (%d%%)",
		strconv.FormatInt(usedGPUMemInCluster, 10),
		strconv.FormatInt(totalGPUMemInCluster, 10),
		int64(gpuUsage))
	// log.Info( "%s\t%s\t%s\t%s\t%s\n", ...)

	//_ = w.Flush()
}

func getMaxGPUCount(nodeInfos []*NodeInfo) (max int) {
	for _, node := range nodeInfos {
		if node.gpuCount > max {
			max = node.gpuCount
		}
	}

	return max
}

func displaySummary(nodeInfos []*NodeInfo) {
	//w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var (
		maxGPUCount          int
		totalGPUMemInCluster int64
		usedGPUMemInCluster  int64
		prtLineLen           int
	)
	totalGPUMemInCluster = 0
	usedGPUMemInCluster = 0
	hasPendingGPU := hasPendingGPUMemory(nodeInfos)

	maxGPUCount = getMaxGPUCount(nodeInfos)

	var buffer bytes.Buffer
	buffer.WriteString("NAME\tIPADDRESS\t")
	for i := 0; i < maxGPUCount; i++ {
		buffer.WriteString(fmt.Sprintf("GPU%d(Allocated/Total)\t", i))
	}

	if hasPendingGPU {
		buffer.WriteString("PENDING(Allocated)\t")
	}
	buffer.WriteString(fmt.Sprintf("GPU Memory(%s)", memoryUnit))

	// log.Info( "NAME\tIPADDRESS\tROLE\tGPU(Allocated/Total)\tPENDING(Allocated)\n")
	log.Info(buffer.String())
	for _, nodeInfo := range nodeInfos {
		address := "unknown"
		if len(nodeInfo.node.Status.Addresses) > 0 {
			// address = nodeInfo.node.Status.Addresses[0].Address
			for _, addr := range nodeInfo.node.Status.Addresses {
				if v1.NodeInternalIP == addr.Type {
					address = addr.Address
					break
				}
			}
		}

		gpuMemInfos := []string{}
		pendingGPUMemInfo := ""
		usedGPUMemInNode := 0
		totalGPUMemInNode := nodeInfo.gpuTotalMemory
		if totalGPUMemInNode <= 0 {
			continue
		}

		for i := 0; i < maxGPUCount; i++ {
			gpuMemInfo := "0/0"
			if dev, ok := nodeInfo.devs[i]; ok {
				gpuMemInfo = dev.String()
				usedGPUMemInNode += dev.usedGPUMem
			}
			gpuMemInfos = append(gpuMemInfos, gpuMemInfo)
		}

		// check if there is pending dev
		if dev, ok := nodeInfo.devs[-1]; ok {
			pendingGPUMemInfo = fmt.Sprintf("%d", dev.usedGPUMem)
			usedGPUMemInNode += dev.usedGPUMem
		}

		nodeGPUMemInfo := fmt.Sprintf("%d/%d", usedGPUMemInNode, totalGPUMemInNode)

		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("%s\t%s\t", nodeInfo.node.Name, address))
		for i := 0; i < maxGPUCount; i++ {
			buf.WriteString(fmt.Sprintf("%s\t", gpuMemInfos[i]))
		}
		if hasPendingGPU {
			buf.WriteString(fmt.Sprintf("%s\t", pendingGPUMemInfo))
		}

		buf.WriteString(fmt.Sprintf("%s", nodeGPUMemInfo))
		log.Info(buf.String())

		if prtLineLen == 0 {
			prtLineLen = buf.Len() + 20
		}

		usedGPUMemInCluster += int64(usedGPUMemInNode)
		totalGPUMemInCluster += int64(totalGPUMemInNode)
	}
	// log.Info( "-----------------------------------------------------------------------------------------\n")
	var prtLine bytes.Buffer
	for i := 0; i < prtLineLen; i++ {
		prtLine.WriteString("-")
	}
	prtLine.WriteString("\n")
	log.Info(prtLine.String())

	log.Info("Allocated/Total GPU Memory In Cluster:")
	log.Info("gpu: %s, allocated GPU Memory %s", strconv.FormatInt(totalGPUMemInCluster, 10),
		strconv.FormatInt(usedGPUMemInCluster, 10))
	var gpuUsage float64 = 0
	if totalGPUMemInCluster > 0 {
		gpuUsage = float64(usedGPUMemInCluster) / float64(totalGPUMemInCluster) * 100
	}
	log.Info("%s/%s (%d%%)\t",
		strconv.FormatInt(usedGPUMemInCluster, 10),
		strconv.FormatInt(totalGPUMemInCluster, 10),
		int64(gpuUsage))
	// log.Info( "%s\t%s\t%s\t%s\t%s\n", ...)

	//_ = w.Flush()
}

func getGPUMemoryInPod(pod v1.Pod) int {
	gpuMem := 0
	for _, container := range pod.Spec.Containers {
		if val, ok := container.Resources.Limits[resourceName]; ok {
			gpuMem += int(val.Value())
		}
	}
	return gpuMem
}
