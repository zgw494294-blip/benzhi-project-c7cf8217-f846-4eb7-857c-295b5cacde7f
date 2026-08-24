package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func HashText(text string) string {
	return HashBytes([]byte(strings.ReplaceAll(text, "\r\n", "\n")))
}

func OrderedPages(v *DigitizationVolume) []FacsimilePage {
	byID := make(map[string]FacsimilePage, len(v.Pages))
	for _, page := range v.Pages {
		byID[page.ID] = page
	}
	ordered := make([]FacsimilePage, 0, len(v.Pages))
	for _, id := range v.PageOrder {
		if page, ok := byID[id]; ok {
			ordered = append(ordered, page)
			delete(byID, id)
		}
	}
	rest := make([]FacsimilePage, 0, len(byID))
	for _, page := range byID {
		rest = append(rest, page)
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].Sequence == rest[j].Sequence {
			return rest[i].ID < rest[j].ID
		}
		return rest[i].Sequence < rest[j].Sequence
	})
	return append(ordered, rest...)
}

func DigestPage(page FacsimilePage) PageDigest {
	transcript := HashText(page.Transcription)
	parts := []string{page.ID, page.FolioLabel, strconv.Itoa(page.Sequence), page.SHA256, transcript, strconv.FormatInt(page.Revision, 10)}
	return PageDigest{
		PageID: page.ID, FolioLabel: page.FolioLabel, Sequence: page.Sequence,
		ImageSHA256: page.SHA256, Transcription: transcript,
		Combined: HashText(strings.Join(parts, "\x1f")),
	}
}

func FreezeDigest(v *DigitizationVolume) (string, []PageDigest) {
	pages := OrderedPages(v)
	digests := make([]PageDigest, 0, len(pages))
	parts := []string{v.ID, v.Title, v.EditionNote, v.ShelfMark}
	for _, page := range pages {
		digest := DigestPage(page)
		digests = append(digests, digest)
		parts = append(parts, digest.Combined)
	}
	return HashText(strings.Join(parts, "\x1e")), digests
}

func ManifestDigest(manifest AccessionManifest) string {
	parts := []string{manifest.ID, manifest.VolumeID, manifest.ManifestNumber, manifest.FrozenDigest, manifest.Reviewer, manifest.ReviewNote, manifest.IssuedAt.UTC().Format("2006-01-02T15:04:05.000000000Z")}
	for _, page := range manifest.PageDigests {
		parts = append(parts, page.Combined)
	}
	return HashText(strings.Join(parts, "\x1d"))
}

func VerifyAccessionManifest(volume *DigitizationVolume) bool {
	if volume == nil || volume.Manifest == nil || volume.Manifest.VolumeID != volume.ID {
		return false
	}
	frozenDigest, pageDigests := FreezeDigest(volume)
	manifest := volume.Manifest
	if frozenDigest != volume.FrozenDigest || manifest.FrozenDigest != frozenDigest || len(manifest.PageDigests) != len(pageDigests) {
		return false
	}
	for index := range pageDigests {
		if manifest.PageDigests[index] != pageDigests[index] {
			return false
		}
	}
	return true
}
