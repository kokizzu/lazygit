package staging

import (
	"github.com/jesseduffield/lazygit/pkg/config"
	. "github.com/jesseduffield/lazygit/pkg/integration/components"
)

var DiffContextChange = NewIntegrationTest(NewIntegrationTestArgs{
	Description:  "Change the number of diff context lines while in the staging panel",
	ExtraCmdArgs: []string{},
	Skip:         false,
	SetupConfig:  func(config *config.AppConfig) {},
	SetupRepo: func(shell *Shell) {
		// need to be working with a few lines so that git perceives it as two separate hunks
		shell.CreateFileAndAdd("file1", "1a\n2a\n3a\n4a\n5a\n6a\n7a\n8a\n9a\n10a\n11a\n12a\n13a\n14a\n15a")
		shell.Commit("one")

		shell.UpdateFile("file1", "1a\n2a\n3b\n4a\n5a\n6a\n7a\n8a\n9a\n10a\n11a\n12a\n13b\n14a\n15a")

		// hunk looks like:
		// diff --git a/file1 b/file1
		// index 3653080..a6388b6 100644
		// --- a/file1
		// +++ b/file1
		// @@ -1,6 +1,6 @@
		//  1a
		//  2a
		// -3a
		// +3b
		//  4a
		//  5a
		//  6a
		// @@ -10,6 +10,6 @@
		//  10a
		//  11a
		//  12a
		// -13a
		// +13b
		//  14a
		//  15a
		// \ No newline at end of file
	},
	Run: func(t *TestDriver, keys config.KeybindingConfig) {
		t.Views().Files().
			IsFocused().
			Lines(
				Contains("file1").IsSelected(),
			).
			PressEnter()

		t.Views().Staging().
			IsFocused().
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			Press(keys.Universal.IncreaseContextInDiffView).
			Tap(func() {
				t.ExpectToast(Equals("Changed diff context size to 4"))
			}).
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			Press(keys.Universal.DecreaseContextInDiffView).
			Tap(func() {
				t.ExpectToast(Equals("Changed diff context size to 3"))
			}).
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			Press(keys.Universal.DecreaseContextInDiffView).
			Tap(func() {
				t.ExpectToast(Equals("Changed diff context size to 2"))
			}).
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			Press(keys.Universal.DecreaseContextInDiffView).
			Tap(func() {
				t.ExpectToast(Equals("Changed diff context size to 1"))
			}).
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			PressPrimaryAction().
			Press(keys.Universal.TogglePanel)

		t.Views().StagingSecondary().
			IsFocused().
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			Press(keys.Universal.DecreaseContextInDiffView).
			Tap(func() {
				t.ExpectToast(Equals("Changed diff context size to 0"))
			}).
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			Press(keys.Universal.IncreaseContextInDiffView).
			Tap(func() {
				t.ExpectToast(Equals("Changed diff context size to 1"))
			}).
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			).
			Press(keys.Universal.IncreaseContextInDiffView).
			Tap(func() {
				t.ExpectToast(Equals("Changed diff context size to 2"))
			}).
			SelectedLines(
				Contains(`-3a`),
				Contains(`+3b`),
			)
	},
})
