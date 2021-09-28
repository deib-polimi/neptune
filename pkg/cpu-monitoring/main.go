package main

import (
	linuxproc "github.com/c9s/goprocinfo/linux"
	"k8s.io/klog/v2"
)

func main() {
	stat, err := linuxproc.ReadStat("/proc/stat")
	if err != nil {
		klog.Fatal("stat read fail")
	}

	klog.Infof("%+v", stat.CPUStatAll)
	//klog.Infof("%+v", stat.CPUStats)
	klog.Infof("%+v", stat.Processes)
	klog.Infof("%+v", stat.BootTime)

	s := stat.CPUStatAll
	nonIdle := float64(s.User + s.Nice + s.System + s.IRQ + s.SoftIRQ + s.Steal)
	total := float64(s.Idle) + nonIdle

	klog.Info(nonIdle/total)

}

