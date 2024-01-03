package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	//
	// Uncomment to load all auth plugins

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

func colorString(code int, bold bool) string {
	var makeBold int
	if bold {
		makeBold = 1
	} else {
		makeBold = 0
	}

	return fmt.Sprintf("\033[%d;%d;49m", makeBold, code)
}

func secDiff(s int64) string {
	if s > 2592000 {
		return fmt.Sprintf("%dmo", s/2592000)
	} else if s > 604800 {
		return fmt.Sprintf("%dw", s/604800)
	} else if s > 86400 {
		return fmt.Sprintf("%dd", s/86400)
	} else if s > 3600 {
		return fmt.Sprintf("%dh", s/3600)
	} else if s > 60 {
		return fmt.Sprintf("%dmin", s/60)
	} else {
		return fmt.Sprintf("%ds", s)
	}
}

func checkDeprecations(c *kubernetes.Clientset, n string) bool {
	printPodDetails := false
	summarizeDeprecated := true
	summarizeDeprecatedString := ""
	deprecations := false

	// Should move these
	//var goodColor = colorString(green, false)
	var errorColor = colorString(31, false)
	//var warningColor = colorString(yellow, false)
	var normalColor = colorString(37, false)
	//var darkGray = colorString(30, false)

	// this will only get V1beta1 things, which are deprecated.
	ingresses, _ := c.NetworkingV1beta1().Ingresses(n).List(context.TODO(), metav1.ListOptions{})

	for i := 0; i < len(ingresses.Items); i++ {
		if printPodDetails {
			fmt.Printf(" - %sDEPRECATED INGRESS: %s%s\n", errorColor, ingresses.Items[i].Name, normalColor)
		}
		if summarizeDeprecated {
			summarizeDeprecatedString += fmt.Sprintf("Ingress networking/v1beta1 gone post 1.22: %s/%s\n", n, ingresses.Items[i].Name)
		}
		deprecations = true
	}

	// this will only get V1beta1 things, which are deprecated.
	cronjobs, _ := c.BatchV1beta1().CronJobs(n).List(context.TODO(), metav1.ListOptions{})
	for i := 0; i < len(cronjobs.Items); i++ {
		if printPodDetails {
			fmt.Printf(" - %sDEPRECATED CRONJOB: %s%s\n", errorColor, cronjobs.Items[i].Name, normalColor)
		}
		if summarizeDeprecated {
			summarizeDeprecatedString += fmt.Sprintf("CronJob batch/v1beta1 gone post 1.25: %s/%s\n", n, cronjobs.Items[i].Name)
		}
		deprecations = true
	}

	return deprecations
}

// for x := 0; x < len(pods.Items[i].Status.ContainerStatuses); x++ {
// 	if pods.Items[i].Status.ContainerStatuses[x].RestartCount >= restartLimit && (printPodDetails || printPodDetailsWide) { //&&
// 		//						time.Now().Sub(pods.Items[i].Status.StartTime.Time).Minutes() < lastRestartWarningTime {
// 		fmt.Printf("%s RESTART WARNING: %s in the %s namespace restarted a total of %d times in the last %s! %s %s %s\n",
// 			warningColor,
// 			pods.Items[i].Name,
// 			namespaces.Items[n].Name,
// 			pods.Items[i].Status.ContainerStatuses[x].RestartCount,
// 			secDiff(int64(time.Now().Unix()-pods.Items[i].Status.StartTime.Unix())),
// 			pods.Items[i].Status.Conditions[0].Reason,
// 			pods.Items[i].Status.Conditions[0].Message,
// 			normalColor,
// 		)
// 	}
// }

type clusterDetail struct {
	name          string
	namespaces    map[string]nameSpaceDetail
	nodes         []nodeInstanceType
	version       string
	clientset     kubernetes.Clientset
	dynamicClient dynamic.Interface
	usedCPU       int64
	usedRAM       int64
	masterCPU     int64
	// masterCPU     int64
}

type nameSpaceDetail struct {
	name            string
	ingresses       []ingressInfo
	cronJobs        []cronJobInfo
	pods            []podInfo
	deployments     map[string]deployInfo
	hpas            []hpaInfo
	configMaps      []configMapInfo
	secrets         []secretInfo
	images          map[string]imageInfo
	totalCPURequest int64
	totalRAMRequest int64
	statusSummary   podStatusSummary
	virtualServices []unstructured.Unstructured
	serviceAccounts []v1.ServiceAccount
}

type usageData struct {
	name            string
	totalCPURequest int64
	totalRAMRequest int64
	currentPods     int64
}

type deployInfo struct {
	name            string
	count           int
	totalCPURequest int64
	totalRAMRequest int64
	kind            string
}

type configMapInfo struct {
	name string
	data v1.ConfigMap
}

type secretInfo struct {
	name string
	data v1.Secret
}

type hpaInfo struct {
	name    string
	max     int
	min     int
	desired int
}

type podStatusSummary struct {
	running      int
	pending      int
	completed    int
	failed       int
	crashlooping int
	other        int
}

type ingressInfo struct {
	name         string
	isDeprecated bool
}

type cronJobInfo struct {
	name         string
	isDeprecated bool
}

type podInfo struct {
	count                       int
	name                        string
	reservedMemory, reservedCPU int64
	HostIP                      string
	Phase                       v1.PodPhase
	RestartCount                int32
	podRunningTime              int64
	ownerName                   string
	ownerKind                   string
}

type imageInfo struct {
	count        int
	imageKey     string
	imageName    string
	imageRepo    string
	imageVersion string
}

type nodeInstanceType struct {
	name    string
	count   int
	vCPU    int64
	RAM     int64
	storage int64
	group   []string
}

type nodeType struct {
	name    string
	ip      string
	master  bool
	usedCPU float32
	usedRAM float32
}

type rgb struct {
	red   int
	green int
	blue  int
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func inSlice(s []string, str string) (int, bool) {
	for i, v := range s {
		if v == str {
			return i, true
		}
	}

	return 0, false
}

func barChart(data []float32, description string, width int, suffix string) {
	var str string
	const barStart = "|"
	const barEnd = "|"
	const saucer = "█"
	var total = float32(0)
	var colorStart = 31
	var normalColor = colorString(37, false)

	for i := 0; i < len(data); i++ {
		total = total + data[i]
	}

	for i := 0; i < len(data); i++ {
		repeatAmount := int(data[i] / total * float32(width))
		str = str + fmt.Sprintf("%s%s",
			colorString(colorStart, false),
			strings.Repeat(saucer, repeatAmount),
		)
		colorStart++
	}

	fmt.Printf("%s %s%s%s %s%.2f%s", description, barStart, str, barEnd, normalColor, total, suffix)
}

func makeTagColors(trueColor bool) []rgb {
	var colors []rgb
	colors = append(colors, rgb{red: 204, green: 0, blue: 0})
	colors = append(colors, rgb{red: 78, green: 154, blue: 6})
	colors = append(colors, rgb{red: 196, green: 160, blue: 0})
	colors = append(colors, rgb{red: 114, green: 159, blue: 207})
	colors = append(colors, rgb{red: 117, green: 80, blue: 123})
	colors = append(colors, rgb{red: 6, green: 152, blue: 154})
	colors = append(colors, rgb{red: 230, green: 230, blue: 230})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})
	// colors = append(colors, rgb{red:   0,green:   0,blue:   0})

	return colors
}

func main() {
	var kubeConfig *string
	var kubeContext string
	var printPodDetails bool
	const printPodDetailsWide bool = false
	var threadCount int
	var printImageDetails bool
	var printNodeSummary bool
	var trueColor bool
	var debugPrints bool
	// var multiCluster bool
	var summarizeDeprecated bool
	var summarizeDeprecatedString string
	//var wg sync.WaitGroup

	// var showHelp bool
	const restartLimit = 20
	const lastRestartWarningTime = 20
	const dark = 30
	const light = 37
	const red = 31
	const green = 32
	const yellow = 33
	var goodColor = colorString(green, false)
	var errorColor = colorString(red, false)
	//var warningColor = colorString(yellow, false)
	var normalColor = colorString(light, false)
	var darkGray = colorString(dark, false)
	var tagColors []rgb
	var imageMap = make(map[string]imageInfo)
	var podCounter = make(map[string]podInfo)
	var deployAggregateDetails = make(map[string]deployInfo)
	// var nsDetails = make(map[string]nameSpaceDetail)
	var emptyNSDetails = make(map[string]nameSpaceDetail)
	//var podRestarts = make(map[string]podInfo)
	var nsNames []string
	// var clusterName string
	var overallPodStatuses []podStatusSummary
	var emptyNamespaces int
	var maxNameSpaceNameLength = 0
	//var wg sync.WaitGroup
	//var mutex = &sync.Mutex{}
	var clusterDetails []clusterDetail
	var kubeContexts []string
	var clusterCount int
	// var clusters []string

	// clusterName = "tohsaka"

	home := homeDir()

	if home != "" {
		//		flag.StringVar(kubeConfig, "C", filepath.Join(home, ".kube", "config"), "kubernetes Config file (default: ~/.kube/config)")
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	// if they pass in -h it'll say it's not defined and show the autogenerated help text.
	flag.BoolVar(&debugPrints, "D", false, "(optional) Debug Prints Everywhere")
	flag.BoolVar(&printPodDetails, "p", false, "(optional) List details about all pods")
	flag.BoolVar(&printImageDetails, "i", false, "(optional) Print breakdown of Images used in the cluster")
	flag.BoolVar(&printNodeSummary, "n", false, "(optional) Print Summary of Nodes")
	flag.BoolVar(&trueColor, "t", true, "(optional) Use TrueColor terminal support (default: On)")
	flag.BoolVar(&summarizeDeprecated, "d", false, "(optional) Show a list of deprecated issues found at the end")
	flag.IntVar(&threadCount, "T", 3, "(optional) Max Concurrent Threads (default: 3)")
	flag.StringVar(&kubeContext, "c", "", "(optional) Kubernetes Context to use")
	flag.Parse()

	tagColors = makeTagColors(trueColor)
	var progressBar *progressbar.ProgressBar

	kubeContexts = strings.Split(kubeContext, ",")

	if len(kubeContexts) > 0 {
		clusterCount = len(kubeContexts)
		//multiCluster = true
	} else {
		clusterCount = 1
		//multiCluster = false
	}

	// Initialize Some Arrays
	for clusterNum := 0; clusterNum < clusterCount; clusterNum++ {
		overallPodStatuses = append(overallPodStatuses, podStatusSummary{
			running:      0,
			pending:      0,
			completed:    0,
			failed:       0,
			crashlooping: 0,
			other:        0,
		})
	}

	deployNameWidth := 0

	for clusterNum := 0; clusterNum < clusterCount; clusterNum++ {
		var contextToUse string

		configOverrides := &clientcmd.ConfigOverrides{}

		if len(kubeContexts[clusterNum]) > 0 {
			contextToUse = kubeContexts[clusterNum]
			configOverrides = &clientcmd.ConfigOverrides{CurrentContext: contextToUse}
		}

		configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeConfig}

		config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides).ClientConfig()
		if err != nil {
			panic(err.Error())
		}

		// create the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
		//discovery := clientset.DiscoveryClient
		// fmt.Println(discovery.ServerVersion())

		//fmt.Printf("Connected to cluster running %s\n", discovery.ServerVersion())

		// for cluster < len(clusters) {

		// for nsgroup := 0; nsgroup < threadCount && nstotal < len(namespaces.Items); nsgroup++ {
		// wg.Add(1)
		// go func(nsName string) {
		// 	defer wg.Done()

		var currentCluster clusterDetail
		currentCluster.name = "First" // Can I get this somewhere? Context?
		currentCluster.clientset = *clientset
		currentCluster.dynamicClient = dynamicClient
		// wg.Add(1)
		// go func(nsName string) {
		// 	defer wg.Done()
		fmt.Println("Scanning Clusters...")
		progressBar = progressbar.Default(int64(clusterCount * 100))
		currentCluster.namespaces = scanClusterNamespaces(clientset, dynamicClient, progressBar)
		//currentCluster.nodes = scanClusterPods(clientset, dynamicClient)

		//nodes, _ := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		//currentCluster.nodes = nodes

		clusterDetails = append(clusterDetails, currentCluster)

		fmt.Printf("Found %d namespaces\n", len(clusterDetails[0].namespaces))

		// grab info for Namespace
		// mutex.Lock()
		//	nsDetails[nsName] = clusterDetails[nsName] //scanNamespace(clientset, dynamicClient, nsName)
		//  printNameSpaceDetails(scanNamespace(clientset, nsName), 0, 0)
		// 	mutex.Unlock()
		//
		// }(namespaces.Items[nstotal].Name)
		// }
		// wg.Wait()
		//}
	}

	for clusterNum := 0; clusterNum < clusterCount; clusterNum++ {
		var nsTotalRAM int64
		var nsTotalCPU int64
		var nstotal = 0

		for _, ns := range clusterDetails[clusterNum].namespaces {
			nstotal++
			if len(ns.name) > maxNameSpaceNameLength {
				maxNameSpaceNameLength = len(ns.name)
			}

			// // Check for deprecations
			// if summarizeDeprecated {
			// 	checkDeprecations(clientset, ns.name)
			// }
			nsNames = append(nsNames, ns.name)

			for p := 0; p < len(ns.pods); p++ {
				podCounter[ns.pods[p].HostIP] = podInfo{
					count:          podCounter[ns.pods[p].HostIP].count + 1,
					reservedMemory: podCounter[ns.pods[p].HostIP].reservedMemory + ns.pods[p].reservedMemory,
					reservedCPU:    podCounter[ns.pods[p].HostIP].reservedCPU + ns.pods[p].reservedCPU,
				}
			}

			// No Pods, No Ingresses, No VirtualServices ... Probably should add more things here, or
			// a function... yeah, a function.
			//fmt.Printf("%s: secrets: %d\t configmaps: %d", ns.name, len(ns.secrets), len(ns.configMaps))
			thingsInNameSpace := len(ns.pods) + len(ns.virtualServices) + len(ns.ingresses) + len(ns.configMaps) + len(ns.secrets) + len(ns.cronJobs)
			if thingsInNameSpace == 0 {
				emptyNamespaces++
				emptyNSDetails[ns.name] = ns
			}

			// pull nsDetail info into overall imageMap
			for _, info := range ns.images {
				if _, ok := imageMap[info.imageKey]; ok {
					// increment
					imageMap[info.imageKey] = imageInfo{
						count:        imageMap[info.imageKey].count + 1,
						imageName:    info.imageName,
						imageRepo:    info.imageRepo,
						imageVersion: info.imageVersion,
					}
				} else {
					imageMap[info.imageKey] = imageInfo{
						imageName:    info.imageName,
						imageVersion: info.imageVersion,
						imageRepo:    info.imageRepo,
						count:        1,
					}
				}

				// pull nsDetail info into overall deployAggregateDetails
				for _, info := range ns.deployments {
					thisWidth := len(info.name)
					if thisWidth > deployNameWidth {
						deployNameWidth = thisWidth
					}
					if _, ok := deployAggregateDetails[info.name]; ok {
						// increment
						deployAggregateDetails[info.name] = deployInfo{
							count:           deployAggregateDetails[info.name].count + info.count,
							totalCPURequest: deployAggregateDetails[info.name].totalCPURequest + info.totalCPURequest,
							totalRAMRequest: deployAggregateDetails[info.name].totalRAMRequest + info.totalRAMRequest,
						}
					} else {
						deployAggregateDetails[info.name] = deployInfo{
							name:            info.name,
							count:           info.count,
							totalCPURequest: info.totalCPURequest,
							totalRAMRequest: info.totalRAMRequest,
						}
					}
				}
				//fmt.Printf("There are %d pods in the namespace %s\n", len(pods.Items), namespaces.Items[n].Name)

				overallPodStatuses[clusterNum] = podStatusSummary{
					running:      overallPodStatuses[clusterNum].running + ns.statusSummary.running,
					pending:      overallPodStatuses[clusterNum].pending + ns.statusSummary.pending,
					completed:    overallPodStatuses[clusterNum].completed + ns.statusSummary.completed,
					failed:       overallPodStatuses[clusterNum].failed + ns.statusSummary.failed,
					crashlooping: overallPodStatuses[clusterNum].crashlooping + ns.statusSummary.crashlooping,
					other:        overallPodStatuses[clusterNum].other + ns.statusSummary.other,
				}
				nsTotalCPU = nsTotalCPU + ns.totalCPURequest
				nsTotalRAM = nsTotalRAM + ns.totalRAMRequest

				if len(ns.pods) > 0 && printPodDetails {
					printNameSpaceDetails(ns, nsTotalCPU, nsTotalRAM)
				}
			}

			clusterDetails[clusterNum].usedCPU = clusterDetails[clusterNum].usedCPU + nsTotalCPU/1000 // nsTotalCPU is in milliCPU
			clusterDetails[clusterNum].usedRAM = clusterDetails[clusterNum].usedRAM + nsTotalRAM

			progressBar.Add(1)
		}
	} // End Cluster Scans

	// Cluster Pod Breakdown
	for clusterNum := 0; clusterNum < len(clusterDetails); clusterNum++ {
		fmt.Printf("\n - There are %d namespaces, %d of which are empty:\n", len(clusterDetails[clusterNum].namespaces), emptyNamespaces)
		for _, info := range emptyNSDetails {
			if info.name != "" {
				fmt.Printf("   %s· %s%s%s\n", normalColor, colorString(yellow, false), info.name, normalColor)
			}
		}

		var pendingColor = goodColor
		var otherColor = goodColor
		if overallPodStatuses[clusterNum].pending > 0 {
			pendingColor = errorColor
		}
		if overallPodStatuses[clusterNum].pending > 0 || overallPodStatuses[clusterNum].failed > 0 {
			otherColor = errorColor
		}
		if printPodDetails {
			fmt.Printf("\n - Pod Status Breakdown: %d Running - %s%d%s Pending - %s%d%s Failed - %s%d%s Completed - %s%d%s Other\n",
				overallPodStatuses[clusterNum].running,
				pendingColor, overallPodStatuses[clusterNum].pending, normalColor,
				pendingColor, overallPodStatuses[clusterNum].failed, normalColor,
				pendingColor, overallPodStatuses[clusterNum].completed, normalColor,
				otherColor, overallPodStatuses[clusterNum].other, normalColor)
		}
	}

	fmt.Printf("\n%s===== %sDeployment Breakdown%s =====%s\n", darkGray, goodColor, darkGray, normalColor)
	for Deployment, info := range deployAggregateDetails {
		fmt.Printf("\t%*d x %*s: %5d vCPU, %4d GiB RAM Requested\n", 3, info.count, deployNameWidth, Deployment, info.totalCPURequest, info.totalRAMRequest/1024/1024/1024)
	}
	fmt.Println()

	if printImageDetails {
		fmt.Printf("\n%s===== %sImage Breakdown%s =====%s\n", darkGray, goodColor, darkGray, normalColor)
		for _, info := range imageMap {
			if info.imageName != "" {
				fmt.Printf(" %s- %s%4d %48s: %48s\n", darkGray, goodColor, info.count, info.imageName, info.imageVersion)
			}
		}
	} // End printImageDetails

	for clusterNum := 0; clusterNum < len(clusterDetails); clusterNum++ {

		if printNodeSummary {
			nodes, _ := clusterDetails[clusterNum].clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			var defaultColor string = "\033[38;2;192;192;192m"
			var bold = false
			var color = 37
			var nameWidth = 0
			var instanceNameWidth = 0
			var totalCores int64 = 0
			var totalRAM int64 = 0
			var totalVolumes = 0
			var typeBreakdown = make(map[string]nodeInstanceType)

			var workload_types []string
			var workload_typeWidth = 0

			// Figure out widths and counts
			for i := 0; i < len(nodes.Items); i++ {
				labels := nodes.Items[i].GetLabels()

				// record taints
				for t := 0; t < len(nodes.Items[i].Spec.Taints); t++ {
					if nodes.Items[i].Spec.Taints[t].Key == "workload_type" {
						curKey := nodes.Items[i].Spec.Taints[t].Value
						if !contains(workload_types, curKey) {
							if len(nodes.Items[i].Spec.Taints[t].Value) > workload_typeWidth {
								workload_typeWidth = len(nodes.Items[i].Spec.Taints[t].Value)
							}
							workload_types = append(workload_types, nodes.Items[i].Spec.Taints[t].Value)

						}
					}
				}
				//			nodeGroup := labels["eks.amazonaws.com/nodegroup"]

				thisWidth := len(nodes.Items[i].Status.Addresses[1].Address)
				if thisWidth > nameWidth {
					nameWidth = thisWidth
				}

				thisInstanceWidth := len(labels["beta.kubernetes.io/instance-type"])
				if thisInstanceWidth > instanceNameWidth {
					instanceNameWidth = thisInstanceWidth
				}

				cores, _ := nodes.Items[i].Status.Capacity.Cpu().AsInt64()
				totalCores = totalCores + cores
				RAM, _ := nodes.Items[i].Status.Capacity.Memory().AsInt64()
				totalRAM = totalRAM + RAM
				storage, _ := nodes.Items[i].Status.Capacity.Storage().AsInt64()
				// I thought this would include nvme, but it does not.
				//storageEphemeral, _ := nodes.Items[i].Status.Capacity.StorageEphemeral().AsInt64()
				//storage = storage + storageEphemeral
				totalVolumes += len(nodes.Items[i].Status.VolumesAttached)

				if _, ok := typeBreakdown[labels["beta.kubernetes.io/instance-type"]]; ok {
					// increment
					typeBreakdown[labels["beta.kubernetes.io/instance-type"]] = nodeInstanceType{
						name:    labels["beta.kubernetes.io/instance-type"],
						count:   typeBreakdown[labels["beta.kubernetes.io/instance-type"]].count + 1,
						vCPU:    cores,
						RAM:     RAM,
						storage: storage,
						//					group:   append(typeBreakdown[labels["beta.kubernetes.io/instance-type"]].group, nodeGroup),
					}
				} else {
					typeBreakdown[labels["beta.kubernetes.io/instance-type"]] = nodeInstanceType{
						name:    labels["beta.kubernetes.io/instance-type"],
						count:   1,
						vCPU:    cores,
						RAM:     RAM,
						storage: storage,
						//					group:   appendIfUnique(typeBreakdown[labels["beta.kubernetes.io/instance-type"]].group, nodeGroup),
					}
				}

			}

			// // check for bad helm secrets:
			// fmt.Printf("\n%s===== %sBad Helm Secrets%s =====%s\n", darkGray, goodColor, darkGray, normalColor)
			// for s := 0; s < len(ns.secrets); s++ {
			// 	secret := ns.secrets[s]
			// 	if secret.data.GetObjectMeta().GetLabels()["status"] == "pending-update" {
			// 		fmt.Printf("%s%s: %s%s\n", errorColor, secret.data.GetName(), secret.data.GetObjectMeta().GetLabels()["status"], normalColor)
			// 	}
			// }

			fmt.Printf("\n%s===== %sInstance Type Breakdown%s =====%s\n", darkGray, goodColor, darkGray, normalColor)
			for _, info := range typeBreakdown {
				fmt.Printf("\t%*d x %*s: %3d vCPU, %3d GiB RAM, %4d GiB local storage\n", 3, info.count, instanceNameWidth, info.name, info.vCPU, info.RAM/1024/1024/1024, info.storage)
			}
			fmt.Println()

			fmt.Printf("\n%s===== %sNode Breakdown%s =====%s\n", darkGray, goodColor, darkGray, normalColor)
			detail := fmt.Sprintf(" There are %d nodes, using %d Volumes in the cluster with a total of %d/%d Cores and %d/%d GB of RAM ", len(nodes.Items), totalVolumes, clusterDetails[clusterNum].usedCPU, totalCores, clusterDetails[clusterNum].usedRAM/1073741824, totalRAM/1073741824)
			fmt.Println(detail)
			var detailLine string
			for i := 0; i < len(detail); i++ {
				detailLine = detailLine + "-"
			}
			fmt.Printf("%s\n%s     ", detailLine, defaultColor)

			for c, info := range workload_types {
				typeColor := fmt.Sprintf("\033[38;2;%d;%d;%dm", tagColors[c].red, tagColors[c].green, tagColors[c].blue)
				fmt.Printf("%s  ⬤  %*s", typeColor, workload_typeWidth, info)
			}
			fmt.Printf("\n\n")

			for i := 0; i < len(nodes.Items); i++ {
				var nodeColorStripe string
				var azColor string
				var taintColor string
				var workloadColor string = colorString(37, false)
				var capacityTypeColor string
				var tcBackground = (i%2)*15 + 25
				var timeToLive string = ""
				var instanceType string

				warnings := ""
				create := nodes.Items[i].CreationTimestamp.Unix()
				capacity := nodes.Items[i].Status.Capacity
				totalMemorySize := resource.MustParse(capacity.Memory().String())
				hostName := nodes.Items[i].Status.Addresses[1].Address
				ipAddress := nodes.Items[i].Status.Addresses[0].Address
				labels := nodes.Items[i].GetLabels()
				Taints := nodes.Items[i].Spec.Taints
				instanceType = labels["beta.kubernetes.io/instance-type"]
				nodeGroup := "default"
				nodeColors := rgb{
					red:   192,
					green: 192,
					blue:  192,
				}

				for j := 0; j < len(Taints); j++ {
					taint := Taints[j]
					if taint.Key == "workload_type" {
						nodeGroup = taint.Value
					}
				}

				capacityType, exist := labels["eks.amazonaws.com/capacityType"]
				if exist && capacityType == "ON_DEMAND" {
					bold = true
				} else {
					bold = false
				}

				zone, exist := labels["failure-domain.beta.kubernetes.io/zone"]
				if exist {
					color = int(zone[len(zone)-1]) - int('a') + 33
				} else {
					color = 37
				}

				normalColor := colorString(37, false)
				//nodeColor = colorString(37, false)
				if nodes.Items[i].Spec.Unschedulable {
					//nodeColor = colorString(31, true)
					warnings = warnings + "🚫 "
				}
				// TODO Warn if:
				//taint: DeletionCandidateOfClusterAutoscaler is set

				// Default
				workloadColor = fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm", 192, 192, 192, tcBackground, tcBackground, tcBackground)

				for t := 0; t < len(nodes.Items[i].Spec.Taints); t++ {
					if nodes.Items[i].Spec.Taints[t].Key == "workload_type" {
						if c, ok := inSlice(workload_types, nodes.Items[i].Spec.Taints[t].Value); ok {
							if trueColor {
								workloadColor = fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm", tagColors[c].red, tagColors[c].green, tagColors[c].blue, tcBackground, tcBackground, tcBackground)
							}
						}
					}
					//    | Taint ToBeDeletedByClusterAutoscaler => 1663865519
					if nodes.Items[i].Spec.Taints[t].Key == "DeletionCandidateOfClusterAutoscaler" {
						warnings = warnings + "🗑 "
						//timeToDie, _ := strconv.ParseInt(nodes.Items[i].Spec.Taints[t].Value, 10, 64)
						//timeToLive = fmt.Sprintf("🗑  in %s", secDiff(time.Now().Unix()-timeToDie))
					}
					if nodes.Items[i].Spec.Taints[t].Key == "node.kubernetes.io/not-ready" {
						warnings = warnings + "✨ "
					}
					if nodes.Items[i].Spec.Taints[t].Key == "node.kubernetes.io/disk-pressure" {
						warnings = warnings + "💾 "
					}
					if nodes.Items[i].Spec.Taints[t].Key == "eks.amazonaws.com/compute-type" {
						instanceType = "Fargate  "
					} //=> fargate ???

					//}
				}
				if trueColor {
					nodeColorStripe = fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm", nodeColors.red, nodeColors.green, nodeColors.blue, tcBackground, tcBackground, tcBackground)
					capacityTypeColor = fmt.Sprintf("\033[38;2;192;192;192;48;2;%d;%d;%d;1m", tcBackground, tcBackground, tcBackground)
					taintColor = fmt.Sprintf("\033[38;2;164;164;164;48;2;%d;%d;%dm", tcBackground, tcBackground, tcBackground)
					azColor = fmt.Sprintf("\033[%d;48;2;%d;%d;%dm", color, tcBackground, tcBackground, tcBackground)
				} else {
					nodeColorStripe = fmt.Sprintf("%s%s", colorString(37, (i%2 == 0)), colorString(7, (i%2 == 0)))
					azColor = fmt.Sprintf("\033[%dm;", color)
					capacityTypeColor = colorString(37, bold)
					taintColor = normalColor
				}

				//colorCode := colorString(color, bold)
				if exist { // We're FRC or in AWS with the appropriate labels set
					if printPodDetails {
						fmt.Printf("%s%8s %12s %6s old %s%42s%s is %s%9s%s in %s%10s%s & %d taints - %2s CPUs %3v Gi (%12s) %14s, %d Labels %d Vols%s. %3d Pods w/req: %2d vCPU, %3d GiB Mem %s\n",
							nodeColorStripe,
							warnings,
							nodeGroup,
							secDiff(time.Now().Unix()-create),
							workloadColor,
							hostName,
							nodeColorStripe,
							capacityTypeColor,
							labels["eks.amazonaws.com/capacityType"],
							nodeColorStripe,
							azColor,
							labels["failure-domain.beta.kubernetes.io/zone"],
							nodeColorStripe,
							len(nodes.Items[i].Spec.Taints),
							capacity.Cpu().String(),
							totalMemorySize.Value()/1024/1024/1024,
							instanceType,
							ipAddress,
							len(nodes.Items[i].Labels),
							len(nodes.Items[i].Status.VolumesAttached),
							nodeColorStripe,
							podCounter[ipAddress].count,
							podCounter[ipAddress].reservedCPU/1000,
							podCounter[ipAddress].reservedMemory/1024/1024/1024,
							timeToLive,
							//resource.NewQuantity(int64(podCounter[ipAddress].reservedMemory), resource.DecimalSI).ScaledValue(resource.Giga),
						)
					} else { // Normal AWS
						fmt.Printf("%s%6s %6s old %s%*s%s is %s%9s%s in %s%10s%s & %d taints - %2s CPUs %3v Gi (%12s) %14s, %d Labels %d Vol%s %s\n",
							nodeColorStripe,
							warnings,
							secDiff(time.Now().Unix()-create),
							workloadColor,
							nameWidth,
							hostName,
							nodeColorStripe,
							capacityTypeColor,
							labels["eks.amazonaws.com/capacityType"],
							nodeColorStripe,
							azColor,
							labels["failure-domain.beta.kubernetes.io/zone"],
							nodeColorStripe,
							len(nodes.Items[i].Spec.Taints),
							capacity.Cpu().String(),
							totalMemorySize.Value()/1024/1024/1024,
							instanceType,
							ipAddress,
							len(nodes.Items[i].Labels),
							len(nodes.Items[i].Status.VolumesAttached),
							normalColor,
							timeToLive,
						)
					}
				} else { // We're on prem
					if printPodDetails {
						fmt.Printf("%s%s %s%16s%s is %6s old has %d taints - %2s CPUs %3v Gi - %14s, %d Labels %d, Vols%s. %3d Pods w/req: %3d CPUs, %3d Mem\n",
							nodeColorStripe,
							warnings,
							workloadColor,
							hostName,
							nodeColorStripe,
							secDiff(time.Now().Unix()-create),
							len(nodes.Items[i].Spec.Taints),
							capacity.Cpu().String(),
							totalMemorySize.Value()/1024/1024/1024,
							ipAddress,
							len(nodes.Items[i].Labels),
							len(nodes.Items[i].Status.VolumesAttached),
							nodeColorStripe,
							podCounter[ipAddress].count,
							podCounter[ipAddress].reservedCPU/1000,
							podCounter[ipAddress].reservedMemory/1024/1024/1024,
						)
					} else {
						fmt.Printf("%s%s %s%16s%s is %6s old has %d taints - %2s CPUs %3v Gi - %14s, %d Labels, %d Vols %s.\n",
							nodeColorStripe,
							warnings,
							workloadColor,
							hostName,
							nodeColorStripe,
							secDiff(time.Now().Unix()-create),
							len(nodes.Items[i].Spec.Taints),
							capacity.Cpu().String(),
							totalMemorySize.Value()/1024/1024/1024,
							ipAddress,
							len(nodes.Items[i].Labels),
							len(nodes.Items[i].Status.VolumesAttached),
							nodeColorStripe,
						)
					}
				}
				// beta.kubernetes.io/instance-type
				// failure-domain.beta.kubernetes.io/zone

				//		if len(nodes.Items[i].Labels) > 0 {
				//			for j := 0; j < len(nodes.Items[i].Spec); j++ {

				//			}
				//		}
				for j := 0; j < len(nodes.Items[i].Spec.Taints); j++ {
					taint := nodes.Items[i].Spec.Taints[j]
					if taint.Key != "workload_type" &&
						taint.Key != "DeletionCandidateOfClusterAutoscaler" &&
						taint.Key != "node.kubernetes.io/disk-pressure" &&
						taint.Key != "eks.amazonaws.com/compute-type" {
						fmt.Printf("%s    | Taint %s => %s \n", taintColor, taint.Key, taint.Value)
					}
				}
			}
		} // End printNodeSummary
	}
	if summarizeDeprecated && (len(summarizeDeprecatedString) > 0) {
		fmt.Printf("\n%s===== %sDeprecations/Warnings%s =====%s\n", darkGray, errorColor, darkGray, normalColor)
		fmt.Println(summarizeDeprecatedString)
	}

}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func printMap(m []string) string {
	var output string
	for _, info := range m {
		output = output + " " + info
	}
	return output
}

// func appendIfUnique(m []string, newString string) []string {
// 	if {
// 		return m
// 	}
// 	return append(m, newString)
// }
