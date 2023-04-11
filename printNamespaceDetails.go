package main

import (
	"fmt"
)

func printNameSpaceDetails(ns nameSpaceDetail, nsTotalCPU int64, nsTotalRAM int64) {
	const dark = 30
	const light = 37
	const red = 31
	const green = 32
	const yellow = 33
	var goodColor = colorString(green, false)
	var errorColor = colorString(red, false)
	var warningColor = colorString(yellow, false)
	var normalColor = colorString(light, false)
	//var darkGray = colorString(dark, false)
	var statusColor = goodColor
	//	var podMemory []float32
	//	var podCPU []float32

	fmt.Printf("\n%s Namespace %s%s has %d vs, %d cm, %d secrets, and %d pods using %d images with requests of %dm CPU & %d MB RAM\n", normalColor, ns.name, normalColor, len(ns.virtualServices), len(ns.configMaps), len(ns.secrets), len(ns.pods), len(ns.images), ns.totalCPURequest, ns.totalRAMRequest/1024/1024)
	//fmt.Printf("%d pods and %d images found\n", len(ns.pods), len(ns.images))

	// check for bad helm secrets:
	for s := 0; s < len(ns.secrets); s++ {
		secret := ns.secrets[s]
		if secret.data.GetObjectMeta().GetLabels()["status"] == "pending-update" {
			fmt.Printf("%sBad Helm Secret %s: %s%s\n", errorColor, secret.data.GetName(), secret.data.GetObjectMeta().GetLabels()["status"], normalColor)
		}
	}

	for p := 0; p < len(ns.pods); p++ {
		//		podMemory = append(podMemory, float32(ns.pods[p].reservedMemory/1024/1024/1024))
		//		podCPU = append(podMemory, float32(ns.pods[p].reservedCPU))

		switch ns.pods[p].Phase {
		case "Pending":
			// XXX Put reason back when we have it.
			//fmt.Printf("%s PENDING: %s in the %s namespace.%s \n", warningColor, ns.pods[p].name, ns.name, normalColor) //, pods.Items[i].Status.Conditions[0].Reason, pods.Items[i].Status.Conditions[0].Message)
			statusColor = errorColor
		default:
			statusColor = warningColor
		}

		useColor := goodColor
		if ns.pods[p].reservedMemory/1024/1024/512 > 1 {
			useColor = warningColor
		} else if ns.pods[p].reservedCPU/1024/1024/1024 > 2 {
			useColor = warningColor
		}

		fmt.Printf("%s%12s %s %48s - %64s - %s %5dm vCPU  %4dMB MEM\n",
			statusColor,
			ns.pods[p].Phase,
			normalColor,
			ns.name,
			ns.pods[p].name,
			useColor,
			ns.pods[p].reservedCPU,
			ns.pods[p].reservedMemory/1024/1024,
		)
	}
	/*
		for s := 0; s<len(ns.secrets); s++ {
			if ns.secrets[s].data["metadata"]["labels"]["status"]
		}
	*/
}
