// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/muka/arpdns/arp"
	"github.com/muka/ddns/api"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		fullDomain := viper.GetString("domain")

		if viper.GetString("log_level") != "" {
			lvl, err := log.ParseLevel(viper.GetString("log_level"))
			if err != nil {
				fmt.Printf("Failed to parse log level: %s", err)
				os.Exit(1)
			}
			log.SetLevel(lvl)
		}

		conn, err := grpc.Dial(viper.GetString("ddns"), grpc.WithInsecure())
		if err != nil {
			fmt.Printf("Failed to connect to ddns: %s", err)
			os.Exit(1)
		}

		client := api.NewDDNSServiceClient(conn)

		defer conn.Close()

		serviceMapping := viper.GetStringMapStringSlice("addresses")
		response := make(chan [2]string, 1)

		go func() {
			for {
				select {
				case raw := <-response:
					ip := raw[0]
					hwaddr := raw[1]

					log.Debugf("Sync IP %s is at %s", ip, hwaddr)

					ctx1 := context.Background()
					ctx, cancel := context.WithTimeout(ctx1, time.Millisecond*500)

					defer cancel()

					domains, ok := serviceMapping[hwaddr]

					if !ok {
						log.Debugf("Skip %s (%s)", ip, hwaddr)
						continue
					}

					for _, domain := range domains {

						nameOnly := viper.GetBool("name_only")

						record := new(api.Record)
						record.Ip = ip
						record.PTR = true
						record.Domain = domain + fullDomain
						record.TTL = 100
						record.Type = "A"

						_, err = client.SaveRecord(ctx, record)
						if err != nil {
							log.Errorf("Failed to save record %s: %s", record.Domain, err)
						}

						if nameOnly {
							record.Domain = domain
							_, err = client.SaveRecord(ctx, record)
							if err != nil {
								log.Errorf("Failed to save record %s: %s", record.Domain, err)
							}
						}

					}

					break
				}
			}
		}()

		err = arp.ScanARP(response)
		if err != nil {
			fmt.Printf("ARP failed: %s", err)
			os.Exit(1)
		}

	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
