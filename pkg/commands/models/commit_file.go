package models

// CommitFile : A git commit file
type CommitFile struct {
	Name string

	// PatchStatus tells us whether the file has been wholly or partially added to a patch. We might want to pull this logic up into the gui package and make it a map like we do with cherry picked commits
	PatchStatus int // one of 'WHOLE' 'PART' 'NONE'

	ChangeStatus string // e.g. 'A' for added or 'M' for modified. This is based on the result from git diff --name-status
}

func (f *CommitFile) ID() string {
	return f.Name
}

func (f *CommitFile) Description() string {
	return f.Name
}
