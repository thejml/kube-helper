package main

import (
	"context"
	"regexp"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func scanNamespace(c *kubernetes.Clientset, dynamicClient dynamic.Interface, n string) nameSpaceDetail {
	var nsDetails nameSpaceDetail
	nsDetails.images = make(map[string]imageInfo)
	nsDetails.name = n

	// deploy, _ := c.AppsV1().Deployments(n).List(context.TODO(), metav1.ListOptions{})
	// for i := 0; i < len(deploy.Items); i++ {

	// }

	// hpa, _ := c.AutoscalingV1().HorizontalPodAutoscalers(n).List(context.TODO(), metav1.ListOptions{})
	// for i := 0; i < len(hpa.Items); i++ {
	// 	hpa.Items[i].Name()
	// }

	//ing, _ := c.NetworkingV1().IngressClasses().List(context.TODO(), metav1.ListOptions{})

	// Gotta figure out how to do the dynamicClient here.
	//dynamicClient, err := dynamic.NewForConfig(config)

	//  Create a GVR which represents an Istio Virtual Service.
	virtualServiceGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	//  List all of the Virtual Services.
	virtualServices, _ := dynamicClient.Resource(virtualServiceGVR).Namespace(n).List(context.TODO(), metav1.ListOptions{})

	pods, _ := c.CoreV1().Pods(n).List(context.TODO(), metav1.ListOptions{})

	// Pod Loop
	for i := 0; i < len(pods.Items); i++ {
		// Start with empty pod info and then we fill it.
		var podDetails podInfo
		memoryRequests := int64(0)
		cpuRequests := int64(0)

		switch pods.Items[i].Status.Phase {
		case "Running":
			nsDetails.statusSummary.running++
		case "Pending":
			nsDetails.statusSummary.pending++
		case "Failed":
			nsDetails.statusSummary.failed++
		case "Completed":
			nsDetails.statusSummary.completed++
		default:
			nsDetails.statusSummary.other++
		}

		// Container Loop
		for c := 0; c < len(pods.Items[i].Spec.Containers); c++ {
			memoryRequests += int64(pods.Items[i].Spec.Containers[c].Resources.Requests.Memory().Value())
			cpuRequests += int64(pods.Items[i].Spec.Containers[c].Resources.Requests.Cpu().MilliValue())
			image := pods.Items[i].Spec.Containers[c].Image
			// Image in this format: 590528590067.dkr.ecr.us-west-2.amazonaws.com/forrent-etl-java-cronjobs:9a4f4546e04dd3d314d639d7b2e7cc2e15ee2cac or :1.23.59a
			r, _ := regexp.Compile("^([0-9a-zA-Z.-]+)/([-0-9a-zA-Z./]+):([-0-9a-zA-Z.]+)$")

			var matches [3]string
			for index, match := range r.FindStringSubmatch(image) {
				if index > 0 {
					matches[index-1] = match
				}
			}

			if _, ok := nsDetails.images[image]; ok {
				// increment
				nsDetails.images[image] = imageInfo{
					count:        nsDetails.images[image].count + 1,
					imageKey:     image,
					imageName:    matches[1],
					imageRepo:    matches[0],
					imageVersion: matches[2],
				}
			} else {
				nsDetails.images[image] = imageInfo{
					imageName:    matches[1],
					imageKey:     image,
					imageVersion: matches[2],
					imageRepo:    matches[0],
					count:        1,
				}
			}

		} // End Container Loop

		// Move this to be done in the returned location
		// podCounter[pods.Items[i].Status.HostIP] = podInfo{
		// 	count:          podCounter[pods.Items[i].Status.HostIP].count + 1,
		// 	reservedMemory: podCounter[pods.Items[i].Status.HostIP].reservedMemory + int64(memoryRequests),
		// 	reservedCPU:    podCounter[pods.Items[i].Status.HostIP].reservedCPU + int64(cpuRequests),
		// }
		//fmt.Printf(" ===> %20d", podCounter[pods.Items[i].Status.HostIP].reservedMemory/1024/1024)

		maxRestartCount := int32(0)
		podRunningTime := int64(0)
		for x := 0; x < len(pods.Items[i].Status.ContainerStatuses); x++ {
			if pods.Items[i].Status.ContainerStatuses[x].RestartCount > maxRestartCount {
				maxRestartCount = pods.Items[i].Status.ContainerStatuses[x].RestartCount
			}
			runTime := int64(time.Now().Unix() - pods.Items[i].Status.StartTime.Unix())
			if runTime > podRunningTime {
				podRunningTime = runTime
			}

			//			if pods.Items[i].Status.ContainerStatuses[x].RestartCount >= restartLimit && (printPodDetails || printPodDetailsWide) { //&&
			//						time.Now().Sub(pods.Items[i].Status.StartTime.Time).Minutes() < lastRestartWarningTime {
			// fmt.Printf("%s RESTART WARNING: %s in the %s namespace restarted a total of %d times in the last %s! %s %s %s\n",
			// 	warningColor,
			// 	pods.Items[i].Name,
			// 	n,
			// 	pods.Items[i].Status.ContainerStatuses[x].RestartCount,
			// 	secDiff(int64(time.Now().Unix()-pods.Items[i].Status.StartTime.Unix())),
			// 	pods.Items[i].Status.Conditions[0].Reason,
			// 	pods.Items[i].Status.Conditions[0].Message,
			// 	normalColor,
			// )
			//			}
		}

		podDetails = podInfo{
			count:          0,
			name:           pods.Items[i].Name,
			reservedMemory: int64(memoryRequests),
			reservedCPU:    int64(cpuRequests),
			HostIP:         pods.Items[i].Status.HostIP,
			Phase:          pods.Items[i].Status.Phase,
			RestartCount:   maxRestartCount,
			podRunningTime: podRunningTime,
		}
		nsDetails.virtualServices = virtualServices.Items
		nsDetails.totalRAMRequest += int64(memoryRequests)
		nsDetails.totalCPURequest += int64(cpuRequests)
		// And tack it on the nsDetails!
		nsDetails.pods = append(nsDetails.pods, podDetails)
	} // End Pod Loop

	return nsDetails
}
