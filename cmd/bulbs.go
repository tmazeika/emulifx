package cmd

import (
	"github.com/spf13/cobra"
	"github.com/bionicrm/emulifx/server"
	"log"
)

var (
	colorCmd = &cobra.Command{
		Use:"color",
		Short:"emulates the LIFX Color 1000 bulb",
		Run:func(cmd *cobra.Command, args []string) {
			if err := server.Start(label, group, false); err != nil {
				log.Fatalln(err)
			}
		},
	}
	whiteCmd = &cobra.Command{
		Use:"white",
		Short:"emulates the LIFX White 800",
		Run:func(cmd *cobra.Command, args []string) {
			if err := server.Start(label, group, true); err != nil {
				log.Fatalln(err)
			}
		},
	}
)

func init() {
	RootCmd.AddCommand(colorCmd, whiteCmd)
}