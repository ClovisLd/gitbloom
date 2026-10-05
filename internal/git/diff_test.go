package git

import "testing"

func TestReadWorkingFileStaysInRepo(t *testing.T) {
	repo := testRepo(t)
	writeFile(t, repo, "ok.txt", "hello\n")

	if got, err := repo.ReadWorkingFile("ok.txt"); err != nil || got != "hello\n" {
		t.Fatalf("ReadWorkingFile(ok.txt) = %q, %v; want %q, nil", got, err, "hello\n")
	}

	for _, p := range []string{"", "../secret", "..\\secret", "a/../../secret", "/etc/passwd"} {
		if _, err := repo.ReadWorkingFile(p); err == nil {
			t.Fatalf("ReadWorkingFile(%q) should have been rejected", p)
		}
	}
}
