package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	fieldkey "example.com/fieldservice-key-rotation"
)

func main() {
	workOrderID := flag.String("work-order", "WO-1042", "work order to rotate")
	oldValueDeployments := flag.Int("old-value-deployments", 1, "deployments still using the old value")
	flag.Parse()
	client, err := fieldkey.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	report, err := fieldkey.RotateForWorkOrder(context.Background(), client, fieldkey.WorkOrder{
		ID: *workOrderID, PhotoCount: 3, DispatchStatus: "dispatched", TechnicianFollowUp: "confirm replacement after next job",
	}, *oldValueDeployments)
	if err != nil {
		log.Fatal(err)
	}
	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(output))
}
