package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_Execute_HappyPath(t *testing.T) {
	root := &cobra.Command{Use: "root", RunE: wgetMirrorCmd.RunE}
	root.SetArgs([]string{"https://developer.mozilla.org/en-US/docs/Web/HTML", "destination"})
	err := root.Execute()

	assert.Nil(t, err)

	visited, _ := loadAlreadyVisitedFiles("destination")

	assert.Equal(t, 630, len(visited))

	t.Cleanup(func() {
		osErr := os.RemoveAll("destination")

		assert.Nil(t, osErr)
	})
}

func Test_Execute_WithInvalidArgs(t *testing.T) {
	root := &cobra.Command{Use: "root", RunE: wgetMirrorCmd.RunE}
	root.SetArgs([]string{"destination"})

	err := root.Execute()

	assert.NotNil(t, err, errors.New("error: not enough arguments. Please provide a start URL and a destination directory - wgetMirror [startUrl] [destinationDirectory]"))

	root = &cobra.Command{Use: "root", RunE: wgetMirrorCmd.RunE}
	root.SetArgs([]string{"badurl", "destination"})

	err = root.Execute()

	assert.NotNil(t, err, errors.New("error: invalid start URL"))

	t.Cleanup(func() {
		osErr := os.RemoveAll("destination")

		assert.Nil(t, osErr)
	})
}
