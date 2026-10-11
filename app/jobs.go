package main

import (
	"fmt"
	"io"
	"syscall"
)

type BackgroundJob struct {
	pid     int
	index   int
	command string
}

var (
	backgroundJobs  = make([]BackgroundJob, 0)
	latestJob       = -1
	secondLatestJob = -1
)

func handleBackgroundJobs(printDoneJobs bool, stdin io.Reader, stdout io.Writer) {
	updatedBackgroundJobs := make([]BackgroundJob, 0)
	latestRunningJob := -1
	secondLatestRunningJob := -1

	for _, job := range backgroundJobs {
		marker := " "
		switch job.index {
		case latestJob:
			marker = "+"
		case secondLatestJob:
			marker = "-"
		}

		var status syscall.WaitStatus
		wpid, err := syscall.Wait4(job.pid, &status, syscall.WNOHANG, nil)

		deleteJob := false
		jobStatus := "Running"

		if err != nil || wpid == job.pid {
			jobStatus = "Done"
			deleteJob = true
		}
		if !printDoneJobs {
			fmt.Fprintf(stdout, "[%d]%s  %-24s%s\n", job.index, marker, jobStatus, job.command)
		} else if jobStatus == "Done" {
			fmt.Fprintf(stdout, "[%d]%s  %-24s%s\n", job.index, marker, jobStatus, job.command)
		}

		if !deleteJob {
			updatedBackgroundJobs = append(updatedBackgroundJobs, job)

			secondLatestRunningJob = latestRunningJob
			latestRunningJob = job.index
		}
	}

	backgroundJobs = updatedBackgroundJobs
	latestJob = latestRunningJob
	secondLatestJob = secondLatestRunningJob
}
