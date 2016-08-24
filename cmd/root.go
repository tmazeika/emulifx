package cmd

import "github.com/spf13/cobra"

var (
	RootCmd = &cobra.Command{
		Use:"emulifx",
		Short:"Emulate a LIFX bulb on the network",
	}

	// Flags.

	label string
	group string
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&label, "label", "l", "MyDevice",
		"the initial label of the device")
	RootCmd.PersistentFlags().StringVarP(&group, "group", "g", "MyGroup",
		"the initial group of the device")
}
