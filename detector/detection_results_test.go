package detector

import (
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"strings"
	mock "talisman/internal/mock/prompt"
	"talisman/prompt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDetectionResultsAreSuccessful(t *testing.T) {
	results := NewDetectionResults()
	assert.True(t, results.Successful(), "New detection result is always expected to succeed")
	assert.False(t, results.HasFailures(), "New detection result is not expected to fail")
}

func TestCallingFailOnDetectionResultsFails(t *testing.T) {
	results := NewDetectionResults()
	results.Fail("some_filename", "filename", "Bomb", []string{})
	assert.False(t, results.Successful(), "Calling fail on a result should not make it succeed")
	assert.True(t, results.HasFailures(), "Calling fail on a result should make it fail")
}

func TestCanRecordMultipleErrorsAgainstASingleFile(t *testing.T) {
	results := NewDetectionResults()
	results.Fail("some_filename", "filename", "Bomb", []string{})
	results.Fail("some_filename", "filename", "Complete & utter failure", []string{})
	results.Fail("another_filename", "filename", "Complete & utter failure", []string{})
	assert.Len(t, results.GetFailures("some_filename"), 2, "Expected two errors against some_filename.")
	assert.Len(t, results.GetFailures("another_filename"), 1, "Expected one error against another_filename")
}

func TestResultsReportsFailures(t *testing.T) {
	results := NewDetectionResults()
	results.Fail("some_filename", "", "Bomb", []string{})
	results.Fail("some_filename", "", "Complete & utter failure", []string{})
	results.Fail("another_filename", "", "Complete & utter failure", []string{})

	actualErrorReport := results.ReportFileFailures("some_filename")
	firstErrorMessage := strings.Join(actualErrorReport[0], " ")
	secondErrorMessage := strings.Join(actualErrorReport[1], " ")
	finalStringMessage := firstErrorMessage + " " + secondErrorMessage

	assert.Regexp(t, "some_filename", finalStringMessage, "Error report does not contain expected output")
	assert.Regexp(t, "Bomb", finalStringMessage, "Error report does not contain expected output")
	assert.Regexp(t, "Complete & utter failure", finalStringMessage, "Error report does not contain expected output")
}

// Presently not showing the ignored files in the log
// func TestLoggingIgnoredFilesDoesNotCauseFailure(t *testing.T) {
// 	results := NewDetectionResults()
// 	results.Ignore("some_file", "some-detector")
// 	results.Ignore("some/other_file", "some-other-detector")
// 	results.Ignore("some_file_ignored_for_multiple_things", "some-detector")
// 	results.Ignore("some_file_ignored_for_multiple_things", "some-other-detector")
// 	assert.True(t, results.Successful(), "Calling ignore should keep the result successful.")
// 	assert.True(t, results.HasIgnores(), "Calling ignore should be logged.")
// 	assert.False(t, results.HasFailures(), "Calling ignore should not cause a result to fail.")

// 	assert.Regexp(t, "some_file was ignored by .talismanrc for the following detectors: some-detector", results.Report(), "foo")
// 	assert.Regexp(t, "some/other_file was ignored by .talismanrc for the following detectors: some-other-detector", results.Report(), "foo")
// 	assert.Regexp(t, "some_file_ignored_for_multiple_things was ignored by .talismanrc for the following detectors: some-detector, some-other-detector", results.Report(), "foo")
// }

func TestTalismanRCSuggestionWhenThereAreFailures(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	prompter := mock.NewMockPrompt(ctrl)
	results := NewDetectionResults()

	// Creating temp file with some content
	fs := afero.NewMemMapFs()
	file, err := afero.TempFile(fs, "", "talismanrc")
	assert.NoError(t, err)
	ignoreFile := file.Name()

	existingContent := `fileignoreconfig:
- filename: existing.pem
  checksum: 123444ddssa75333b25b6275f97680604add51b84eb8f4a3b9dcbbc652e6f27ac
  ignore_detectors: []
scopeconfig: []
`
	err = afero.WriteFile(fs, ignoreFile, []byte(existingContent), 0666)
	assert.NoError(t, err)

	// The tests below depend on the upper configuration which is shared across all three of them. Hence the order in
	// which they run matters.
	t.Run("should not prompt if there are no failures", func(t *testing.T) {
		promptContext := prompt.NewPromptContext(true, prompter)
		prompter.EXPECT().Confirm(gomock.Any()).Return(false).Times(0)

		results.Report(fs, ignoreFile, promptContext)
		bytesFromFile, err := afero.ReadFile(fs, ignoreFile)

		assert.NoError(t, err)
		assert.Equal(t, existingContent, string(bytesFromFile))
	})

	t.Run("when user declines, entry should not be added to talismanrc", func(t *testing.T) {
		promptContext := prompt.NewPromptContext(true, prompter)
		prompter.EXPECT().Confirm("Do you want to add this entry in talismanrc ?").Return(false)
		results.Fail("some_file.pem", "filecontent", "Bomb", []string{})

		results.Report(fs, ignoreFile, promptContext)
		bytesFromFile, err := afero.ReadFile(fs, ignoreFile)

		assert.NoError(t, err)
		assert.Equal(t, existingContent, string(bytesFromFile))
	})

	t.Run("when interactive flag is set to false, it should not ask user", func(t *testing.T) {
		promptContext := prompt.NewPromptContext(false, prompter)
		prompter.EXPECT().Confirm(gomock.Any()).Return(false).Times(0)
		results.Fail("some_file.pem", "filecontent", "Bomb", []string{})

		results.Report(fs, ignoreFile, promptContext)
		bytesFromFile, err := afero.ReadFile(fs, ignoreFile)

		assert.NoError(t, err)
		assert.Equal(t, existingContent, string(bytesFromFile))
	})

	t.Run("when user confirms, entry should be appended to given ignore file", func(t *testing.T) {
		promptContext := prompt.NewPromptContext(true, prompter)
		prompter.EXPECT().Confirm("Do you want to add this entry in talismanrc ?").Return(true)

		results.Fail("some_file.pem", "filecontent", "Bomb", []string{})

		expectedFileContent := `fileignoreconfig:
- filename: existing.pem
  checksum: 123444ddssa75333b25b6275f97680604add51b84eb8f4a3b9dcbbc652e6f27ac
  ignore_detectors: []
scopeconfig: []
fileignoreconfig:
- filename: some_file.pem
  checksum: 87139cc4d975333b25b6275f97680604add51b84eb8f4a3b9dcbbc652e6f27ac
  ignore_detectors: []
scopeconfig: []
`
		results.Report(fs, ignoreFile, promptContext)
		bytesFromFile, err := afero.ReadFile(fs, ignoreFile)

		assert.NoError(t, err)
		assert.Equal(t, expectedFileContent, string(bytesFromFile))
	})

	t.Run("when user confirms for multiple entries, they should be appended to given ignore file", func(t *testing.T) {
		// Clearing file contents from previous tests
		err := afero.WriteFile(fs, ignoreFile, []byte{}, 0666)
		assert.NoError(t, err)

		promptContext := prompt.NewPromptContext(true, prompter)
		prompter.EXPECT().Confirm("Do you want to add this entry in talismanrc ?").Return(true).Times(2)

		results.Fail("some_file.pem", "filecontent", "Bomb", []string{})
		results.Fail("another.pem", "filecontent", "password", []string{})

		expectedFileContent := `fileignoreconfig:
- filename: some_file.pem
  checksum: 87139cc4d975333b25b6275f97680604add51b84eb8f4a3b9dcbbc652e6f27ac
  ignore_detectors: []
- filename: another.pem
  checksum: 117e23557c02cbd472854ebce4933d6daec1fd207971286f6ffc9f1774c1a83b
  ignore_detectors: []
scopeconfig: []
`
		results.Report(fs, ignoreFile, promptContext)
		bytesFromFile, err := afero.ReadFile(fs, ignoreFile)

		assert.NoError(t, err)
		assert.Equal(t, expectedFileContent, string(bytesFromFile))
	})


	err = fs.Remove(ignoreFile)
	assert.NoError(t, err)
}
