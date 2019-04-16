package nvidia

import (
	"fmt"
	"time"

	log "github.com/astaxie/beego/logs"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

var (
	clientTimeout    = 30 * time.Second
	lastAllocateTime time.Time
)

// create docker client
func init() {
	kubeInit()
}

func buildErrResponse(reqs *pluginapi.AllocateRequest, podReqGPU uint) *pluginapi.AllocateResponse {

	responses := pluginapi.AllocateResponse{
		Envs: map[string]string{
			envNVGPU:               fmt.Sprintf("no-gpu-has-%d-to-run", podReqGPU),
			EnvResourceIndex:       fmt.Sprintf("-1"),
			EnvResourceByPod:       fmt.Sprintf("%d", podReqGPU),
			EnvResourceByContainer: fmt.Sprintf("%d", uint(len(reqs.DevicesIDs))),
			EnvResourceByDev:       fmt.Sprintf("%d", getGPUMemory()),
		},
	}
	return &responses
}

// Allocate which return list of devices.
func (m *NvidiaDevicePlugin) Allocate(ctx context.Context,
	reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	//devs := m.devs
	//response := pluginapi.AllocateResponse{
	//	Envs: map[string]string{
	//		"NVIDIA_VISIBLE_DEVICES": strings.Join(reqs.DevicesIDs, ","),
	//	},
	//}
	//
	//for _, id := range reqs.DevicesIDs {
	//	if !deviceExists(devs, id) {
	//		return nil, fmt.Errorf("invalid allocation request: unknown device: %s", id)
	//	}
	//}
	//
	//return &response, nil
	log.Info("request: %v", reqs)
	response := pluginapi.AllocateResponse{}

	log.Info("----Allocating GPU for gpu mem is started----")
	var (
		podReqGPU uint
		found     bool
		assumePod *v1.Pod
	)

	podReqGPU = uint(len(reqs.DevicesIDs))
	log.Info("RequestPodGPUs: %d", podReqGPU)

	m.Lock()
	defer m.Unlock()
	log.Info("checking...")
	pods, err := getCandidatePods()
	if err != nil {
		log.Info("invalid allocation requst: Failed to find candidate pods due to %v", err)
		return buildErrResponse(reqs, podReqGPU), nil
	}

	for _, pod := range pods {
		log.Info("Pod %s in ns %s request GPU Memory %d with timestamp %v",
			pod.Name,
			pod.Namespace,
			getGPUMemoryFromPodResource(pod),
			getAssumeTimeFromPodAnnotation(pod))
	}

	for _, pod := range pods {
		if getGPUMemoryFromPodResource(pod) == podReqGPU {
			log.Info("Found Assumed GPU shared Pod %s in ns %s with GPU Memory %d",
				pod.Name,
				pod.Namespace,
				podReqGPU)
			assumePod = pod
			found = true
			break
		}
	}

	if found {
		id := getGPUIDFromPodAnnotation(assumePod)
		if id < 0 {
			log.Warning("Failed to get the dev ", assumePod)
		}

		candidateDevID := ""
		if id >= 0 {
			ok := false
			candidateDevID, ok = m.GetDeviceNameByIndex(uint(id))
			if !ok {
				log.Warning("Failed to find the dev for pod %v because it's not able to find dev with index %d",
					assumePod,
					id)
				id = -1
			}
		}

		if id < 0 {
			return buildErrResponse(reqs, podReqGPU), nil
		}

		// 1. Create container requests

		reqGPU := uint(len(reqs.DevicesIDs))
		gmem := getGPUMemory()
		response.Envs = map[string]string{
			envNVGPU:               candidateDevID,
			EnvResourceIndex:       fmt.Sprintf("%d", id),
			EnvResourceByPod:       fmt.Sprintf("%d", podReqGPU),
			EnvResourceByContainer: fmt.Sprintf("%d", reqGPU),
			EnvResourceByDev:       fmt.Sprintf("%d", gmem),
			envCUDA:                fmt.Sprintf("%d", id),
			envPerProcGPUMemFract:  fmt.Sprintf("%f", float64(gmem)/float64(reqGPU)),
		}

		// 2. Allocate devices
		devices, err := m.getDevicesFromRequest(reqs.DevicesIDs)
		if err != nil {
			return buildErrResponse(reqs, podReqGPU), err
		}
		response.Devices = devices

		// 3. Update Pod spec
		newPod := updatePodAnnotations(assumePod)
		_, err = clientset.CoreV1().Pods(newPod.Namespace).Update(newPod)
		if err != nil {
			// the object has been modified; please apply your changes to the latest version and try again
			if err.Error() == OptimisticLockErrorMsg {
				// retry
				pod, err := clientset.CoreV1().Pods(assumePod.Namespace).Get(assumePod.Name, metav1.GetOptions{})
				if err != nil {
					log.Warning("Failed due to %v", err)
					return buildErrResponse(reqs, podReqGPU), nil
				}
				newPod = updatePodAnnotations(pod)
				_, err = clientset.CoreV1().Pods(newPod.Namespace).Update(newPod)
				if err != nil {
					log.Warning("Failed due to %v", err)
					return buildErrResponse(reqs, podReqGPU), nil
				}
			} else {
				log.Warning("Failed due to %v", err)
				return buildErrResponse(reqs, podReqGPU), nil
			}
		}

	} else {
		log.Warning("invalid allocation requst: request GPU memory %d can't be satisfied.", podReqGPU)
		// return &responses, fmt.Errorf("invalid allocation requst: request GPU memory %d can't be satisfied", reqGPU)
		return buildErrResponse(reqs, podReqGPU), nil
	}

	log.Info("new allocated GPUs info %v", &response)
	log.Info("----Allocating GPU for gpu mem is ended----")
	// // Add this to make sure the container is created at least
	// currentTime := time.Now()

	// currentTime.Sub(lastAllocateTime)

	return &response, nil
}
func (m *NvidiaDevicePlugin) getDevicesFromRequest(devs []string) ([]*pluginapi.DeviceSpec, error) {
	var devices []*pluginapi.DeviceSpec
	for _, dev := range devs {
		devId, found := m.fakeNameMap[dev]
		if !found {
			log.Error("use error dev: %s", dev)
			return nil, fmt.Errorf("use error dev: %s", dev)
		}
		device := &pluginapi.DeviceSpec{
			ContainerPath: dev,
			HostPath:      devId,
			Permissions:   "rwm",
		}
		devices = append(devices, device)
	}
	return devices, nil
}
