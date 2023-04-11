package main

import (
	"context"
	"regexp"
	"time"

	progressbar "github.com/schollz/progressbar/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type vsMaps struct {
	name string
	vs   []unstructured.Unstructured
}

func scanClusterNamespaces(c *kubernetes.Clientset, dynamicClient dynamic.Interface, progressBar *progressbar.ProgressBar) map[string]nameSpaceDetail {
	var nsDetails = make(map[string]nameSpaceDetail)
	var n string
	// var progressIterator float64
	// var progressValue int64
	//var vs = make(map[string]vsMaps)
	//var configMaps = make(map[string][]configMapInfo)
	//var secrets = make(map[string][]secretInfo)
	//		 Create a GVR which represents an Istio Virtual Service.
	virtualServiceGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  "v1alpha3",
		Resource: "virtualservices",
	}

	//	var ingresses []v1.IngressClassList
	//fmt.Printf("Gathering Data...\n")

	// progressIterator = float64((len(namespaces.Items) + len(virtualServices.Items) + len(secretList.Items) + len(configMapList.Items) + len(pods.Items)) / 1000)
	// progressValue = 0

	namespaces, _ := c.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	for _, ns := range namespaces.Items {
		nsDetails[ns.Name] = nameSpaceDetail{
			name:            ns.Name,
			totalCPURequest: 0,
			totalRAMRequest: 0,
		}
	}
	progressBar.Add(1)
	//	fmt.Printf("%d Done.\n", len(nsDetails))

	//  Gather all of the Virtual Services.
	virtualServices, _ := dynamicClient.Resource(virtualServiceGVR).Namespace("").List(context.TODO(), metav1.ListOptions{})
	for _, v := range virtualServices.Items {
		nsName := v.GetNamespace()
		// var tempvs vsMaps
		// tempvs.name = nsName
		thisNS := nsDetails[nsName]
		thisNS.virtualServices = append(thisNS.virtualServices, v)
		nsDetails[nsName] = thisNS
	}
	progressBar.Add(1)

	// ing, _ := c.NetworkingV1().IngressClasses().List(context.TODO(), metav1.ListOptions{})
	// ing.Items

	//Gather Secrets:
	secretList, _ := c.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})

	for _, s := range secretList.Items {
		nsName := s.GetNamespace()
		var tempsecret secretInfo
		tempsecret.name = s.Name
		tempsecret.data = s
		thisNS := nsDetails[nsName]
		thisNS.secrets = append(thisNS.secrets, tempsecret)
		nsDetails[nsName] = thisNS
	}
	progressBar.Add(1)

	// Gather ConfigMaps
	configMapList, _ := c.CoreV1().ConfigMaps("").List(context.TODO(), metav1.ListOptions{})

	for _, cm := range configMapList.Items {
		nsName := cm.GetNamespace()
		var tempConfigMap configMapInfo
		tempConfigMap.name = cm.Name
		tempConfigMap.data = cm
		thisNS := nsDetails[nsName]
		thisNS.configMaps = append(thisNS.configMaps, tempConfigMap)
		nsDetails[n] = thisNS
	}
	progressBar.Add(1)

	//fmt.Printf("Scanning Pods...")
	pods, _ := c.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})

	for i := 0; i < len(pods.Items); i++ {
		pod := pods.Items[i]
		n = pods.Items[i].Namespace

		var thisNS nameSpaceDetail
		if _, ok := nsDetails[n]; ok {
			thisNS = nsDetails[n]
		}

		thisNS.images = make(map[string]imageInfo)
		thisNS.name = n

		// deploy, _ := c.AppsV1().Deployments(n).List(context.TODO(), metav1.ListOptions{})
		// for i := 0; i < len(deploy.Items); i++ {

		// }

		// hpa, _ := c.AutoscalingV1().HorizontalPodAutoscalers(n).List(context.TODO(), metav1.ListOptions{})
		// for i := 0; i < len(hpa.Items); i++ {
		// 	hpa.Items[i].Name()
		// }

		// Start with empty pod info and then we fill it.
		var podDetails podInfo
		memoryRequests := int64(0)
		cpuRequests := int64(0)

		switch pod.Status.Phase {
		case "Running":
			thisNS.statusSummary.running++
		case "Pending":
			thisNS.statusSummary.pending++
		case "Failed":
			thisNS.statusSummary.failed++
		case "Completed":
			thisNS.statusSummary.completed++
		default:
			thisNS.statusSummary.other++
		}

		// Container Loop
		for c := 0; c < len(pod.Spec.Containers); c++ {
			memoryRequests += int64(pod.Spec.Containers[c].Resources.Requests.Memory().Value())
			cpuRequests += int64(pod.Spec.Containers[c].Resources.Requests.Cpu().MilliValue())
			image := pod.Spec.Containers[c].Image
			// Image in this format: 590528590067.dkr.ecr.us-west-2.amazonaws.com/forrent-etl-java-cronjobs:9a4f4546e04dd3d314d639d7b2e7cc2e15ee2cac or :1.23.59a
			r, _ := regexp.Compile("^([0-9a-zA-Z.-]+)/([-0-9a-zA-Z./]+):([-0-9a-zA-Z.]+)$")

			var matches [3]string
			for index, match := range r.FindStringSubmatch(image) {
				if index > 0 {
					matches[index-1] = match
				}
			}

			if _, ok := thisNS.images[image]; ok {
				// increment
				thisNS.images[image] = imageInfo{
					count:        thisNS.images[image].count + 1,
					imageKey:     image,
					imageName:    matches[1],
					imageRepo:    matches[0],
					imageVersion: matches[2],
				}
			} else {
				thisNS.images[image] = imageInfo{
					imageName:    matches[1],
					imageKey:     image,
					imageVersion: matches[2],
					imageRepo:    matches[0],
					count:        1,
				}
			}

		} // End Container Loop

		maxRestartCount := int32(0)
		podRunningTime := int64(0)
		for x := 0; x < len(pod.Status.ContainerStatuses); x++ {
			if pod.Status.ContainerStatuses[x].RestartCount > maxRestartCount {
				maxRestartCount = pod.Status.ContainerStatuses[x].RestartCount
			}
			runTime := int64(time.Now().Unix() - pod.Status.StartTime.Unix())
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

		/// XXX thisNS needs to "pods[]"... an array like the secrets and configMaps and the like to be added later...

		thisNS.totalRAMRequest += int64(memoryRequests)
		thisNS.totalCPURequest += int64(cpuRequests)
		// And tack it on the nsDetails!
		thisNS.pods = append(thisNS.pods, podDetails)
		nsDetails[n] = thisNS
	} // End Pod Loop
	//fmt.Printf("Done.\n")
	progressBar.Add(1)

	return nsDetails
}
