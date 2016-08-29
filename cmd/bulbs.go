package cmd

import (
	"github.com/bionicrm/emulifx/server"
	"github.com/spf13/cobra"
	"log"
)

var (
	colorCmd = &cobra.Command{
		Use:   "color",
		Short: "emulates the LIFX Color 1000 bulb",
		Run: func(cmd *cobra.Command, args []string) {
			if err := server.Start(addr, true); err != nil {
				log.Fatalln(err)
			}
		},
	}
	whiteCmd = &cobra.Command{
		Use:   "white",
		Short: "emulates the LIFX White 800 bulb",
		Run: func(cmd *cobra.Command, args []string) {
			if err := server.Start(addr, false); err != nil {
				log.Fatalln(err)
			}
		},
	}
)

func init() {
	RootCmd.AddCommand(colorCmd, whiteCmd)
}
