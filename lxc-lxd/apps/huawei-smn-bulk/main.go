package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	smn "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smn/v2"
	smnModel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smn/v2/model"
	smnRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smn/v2/region"

	"sharedmodule"
)

func main() {
	cred, err := basic.NewCredentialsBuilder().WithAk("xxx").WithSk("xxxxx").SafeBuild()
	if err != nil {
		panic(err)
	}

	smnHcClient, err := smn.SmnClientBuilder().WithCredential(cred).WithRegion(smnRegion.AP_SOUTHEAST_4).SafeBuild()
	if err != nil {
		panic(err)
	}

	client := smn.NewSmnClient(smnHcClient)

	for i := 0; i < 1; i++ {

		message := &sharedmodule.Notification{
			Type:    sharedmodule.NotificationTypeEmail,
			Subject: fmt.Sprintf("Test Mantap Djiwa #%d", i),
			Message: "This is a test email #" + fmt.Sprint(i),
			// Replace with your email address
			Receiver: "target@mail.com",
		}

		jsonData, err := json.Marshal(message)
		if err != nil {
			fmt.Printf("Failed to marshal message #%d: %s\n", i, err.Error())
			continue
		}

		m := string(jsonData)
		smnRequest := &smnModel.PublishMessageRequest{
			TopicUrn: "urn:smn:ap-southeast-4:xxxx:notification",
			Body: &smnModel.PublishMessageRequestBody{
				Message: &m,
			},
		}

		_, err = client.PublishMessage(smnRequest)
		if err != nil {
			fmt.Printf("Failed to send message #%d: %s\n", i, err.Error())
		}

		fmt.Printf("Successfully sent message #%d\n", i)

		if i%10 == 0 {
			time.Sleep(5 * time.Second)
		}
	}

}
