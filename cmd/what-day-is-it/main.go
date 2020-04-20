package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/bradhe/what-day-is-it/pkg/clock"
	"github.com/bradhe/what-day-is-it/pkg/logs"
	"github.com/bradhe/what-day-is-it/pkg/server"
	"github.com/bradhe/what-day-is-it/pkg/storage"
	"github.com/bradhe/what-day-is-it/pkg/storage/managers"
	"github.com/bradhe/what-day-is-it/pkg/twilio"
)

var logger = logs.WithPackage("main")

func doSendMessages(managers managers.Managers, sender *twilio.Sender) {
	manager := managers.PhoneNumbers()

	for range time.Tick(15 * time.Minute) {
		logger.Info("starting delivery run")

		for {
			numbers, err := manager.GetNBySendDeadline(10, clock.Clock())

			if err != nil {
				logger.WithError(err).Error("failed to lookup batch for delivery")
				break
			}

			if len(numbers) < 1 {
				break
			}

			for _, number := range numbers {
				if !number.IsSendable {
					logger.Debug("skipping unsendable number")

					// Update this anyway so we don't check it again for a while.
					manager.UpdateSkipped(&number, clock.Clock())

					continue
				}

				body := fmt.Sprintf("Today is %s", clock.GetDayInZone(clock.MustLoadLocation(number.Timezone)))

				if err := sender.Send(number.Number, body); err != nil {
					logger.WithError(err).Warn("failed to deliver message via Twilio")
				}

				// We'll finish this for the day.
				manager.UpdateSent(&number, clock.Clock())
			}
		}
	}
}

func main() {
	var (
		assetBaseDir        = flag.String("asset-base-dir", "pkg/ui/dist", "The directory that assets are built in to.")
		development         = flag.Bool("development", false, "Put the app in development mode. Basically load UI assets from disk instead of memory.")
		twilioAccountSid    = flag.String("twilio-account-sid", "", "The account SID to authenticate with.")
		twilioAuthToken     = flag.String("twilio-auth-token", "", "The Twilio authentication token to authenticate with.")
		twilioPhoneNumber   = flag.String("twilio-phone-number", "", "The Twilio phone number to use when sending messages.")
		cloudformationStack = flag.String("cloudformation-stack", "what-day-is-it-1", "The stack that we want to store data in.")
		addr                = flag.String("addr", "localhost:8081", "Address to bind the server to.")
	)

	flag.Parse()

	if *development {
		logs.EnableDebug()
	} else {
		logs.DisableDebug()
	}

	sender := twilio.NewSender(*twilioAccountSid, *twilioAuthToken, *twilioPhoneNumber)
	managers := storage.New(*cloudformationStack)

	// Start the delivery loop off right!
	go doSendMessages(managers, &sender)

	if err := server.NewServer(managers, &sender, *development, *assetBaseDir).ListenAndServe(*addr); err != nil {
		panic(err)
	}
}
