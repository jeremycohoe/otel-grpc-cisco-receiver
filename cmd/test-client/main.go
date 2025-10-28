package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/jcohoe/otel-grpc-cisco-receiver/proto/generated/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func main() {
	fmt.Println("Cisco Telemetry Test Client")

	// Connect to the receiver
	conn, err := grpc.NewClient("localhost:57500", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGRPCMdtDialoutClient(conn)

	// Create a streaming connection
	stream, err := client.MdtDialout(context.Background())
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	// Create sample telemetry data
	telemetry := &pb.Telemetry{
		NodeId: &pb.Telemetry_NodeIdStr{
			NodeIdStr: "test-switch-1",
		},
		Subscription: &pb.Telemetry_SubscriptionIdStr{
			SubscriptionIdStr: "interface-stats",
		},
		EncodingPath: "/interfaces-ios-xe-oper:interfaces/interface/statistics",
		CollectionId: 12345,
		MsgTimestamp: uint64(time.Now().UnixMilli()),
		DataGpbkv: []*pb.TelemetryField{
			{
				Name: "interface",
				Fields: []*pb.TelemetryField{
					{
						Name: "name",
						ValueByType: &pb.TelemetryField_StringValue{
							StringValue: "GigabitEthernet0/0/1",
						},
					},
					{
						Name: "statistics",
						Fields: []*pb.TelemetryField{
							{
								Name: "rx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{
									Uint64Value: 1234567,
								},
							},
							{
								Name: "tx-pkts",
								ValueByType: &pb.TelemetryField_Uint64Value{
									Uint64Value: 2345678,
								},
							},
							{
								Name: "rx-bytes",
								ValueByType: &pb.TelemetryField_Uint64Value{
									Uint64Value: 987654321,
								},
							},
							{
								Name: "tx-bytes",
								ValueByType: &pb.TelemetryField_Uint64Value{
									Uint64Value: 876543210,
								},
							},
							{
								Name: "admin-status",
								ValueByType: &pb.TelemetryField_StringValue{
									StringValue: "up",
								},
							},
							{
								Name: "oper-status",
								ValueByType: &pb.TelemetryField_StringValue{
									StringValue: "up",
								},
							},
						},
					},
				},
			},
		},
	}

	// Serialize telemetry data
	data, err := proto.Marshal(telemetry)
	if err != nil {
		log.Fatalf("Failed to marshal telemetry: %v", err)
	}

	// Send telemetry data
	req := &pb.MdtDialoutArgs{
		ReqId: 1,
		Data:  data,
	}

	fmt.Printf("Sending telemetry data for node %s...\n", telemetry.GetNodeIdStr())
	err = stream.Send(req)
	if err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}

	// Receive acknowledgment
	resp, err := stream.Recv()
	if err != nil {
		log.Fatalf("Failed to receive response: %v", err)
	}

	fmt.Printf("Received acknowledgment with ReqId: %d\n", resp.ReqId)
	if resp.Errors != "" {
		fmt.Printf("Server reported errors: %s\n", resp.Errors)
	} else {
		fmt.Println("✅ Telemetry data processed successfully!")
	}

	// Send a few more messages to test multiple data points
	for i := 2; i <= 5; i++ {
		// Update metrics with new values
		rxPkts := uint64(1234567 + i*1000)
		txPkts := uint64(2345678 + i*1500)

		telemetry.CollectionId = uint64(12345 + i)
		telemetry.MsgTimestamp = uint64(time.Now().UnixMilli())

		// Update the rx-pkts and tx-pkts values
		telemetry.DataGpbkv[0].Fields[1].Fields[0].ValueByType = &pb.TelemetryField_Uint64Value{Uint64Value: rxPkts}
		telemetry.DataGpbkv[0].Fields[1].Fields[1].ValueByType = &pb.TelemetryField_Uint64Value{Uint64Value: txPkts}

		data, _ = proto.Marshal(telemetry)
		req = &pb.MdtDialoutArgs{
			ReqId: int64(i),
			Data:  data,
		}

		fmt.Printf("Sending update #%d (rx-pkts: %d, tx-pkts: %d)...\n", i, rxPkts, txPkts)
		err = stream.Send(req)
		if err != nil {
			log.Fatalf("Failed to send request %d: %v", i, err)
		}

		resp, err = stream.Recv()
		if err != nil {
			log.Fatalf("Failed to receive response %d: %v", i, err)
		}

		fmt.Printf("✅ Update #%d acknowledged (ReqId: %d)\n", i, resp.ReqId)

		// Small delay between updates
		time.Sleep(100 * time.Millisecond)
	}

	err = stream.CloseSend()
	if err != nil {
		log.Printf("Failed to close send: %v", err)
	}

	fmt.Println("🎉 Test completed successfully! Sent 5 telemetry data points.")
}
