package bookmerge

import (
	"reflect"
	"testing"
)

func TestParseIDsPreservesLargeLongIDsAsIntegers(t *testing.T) {
	ids, err := parseIDs([]string{"9007199254740993", "9007199254740994", "9007199254740993"}, 2, "源书 ID")
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{9007199254740993, 9007199254740994}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%v want=%v", ids, want)
	}
}

func TestMarkDuplicatesFlagsTitleAndContentWithoutExcluding(t *testing.T) {
	chapters := []chapterSnapshot{
		{targetChapterName: " 第一章  开始 ", content: []byte("不同正文")},
		{targetChapterName: "第一章 开始", content: []byte("第二份正文")},
		{targetChapterName: "第二章", content: []byte("不 同 正 文")},
	}
	markDuplicates(chapters)
	if !chapters[0].duplicate || !chapters[1].duplicate || !chapters[2].duplicate {
		t.Fatalf("chapters=%+v", chapters)
	}
	for _, chapter := range chapters {
		if chapter.excluded {
			t.Fatalf("duplicates must remain included by default: %+v", chapter)
		}
	}
	if chapters[0].duplicateReason != "标题重复、正文重复" || chapters[1].duplicateReason != "标题重复" || chapters[2].duplicateReason != "正文重复" {
		t.Fatalf("duplicate reasons=%q,%q,%q", chapters[0].duplicateReason, chapters[1].duplicateReason, chapters[2].duplicateReason)
	}
}

func TestNormalizeTargetRejectsNumericPrecisionUnsafeAuthorInput(t *testing.T) {
	_, _, err := normalizeTarget(TargetBookInput{BookName: "合并书", AuthorID: "9007199254740993.0", CategoryCode: "fantasy", BookStatus: "completed", PublishStatus: "draft"})
	if err == nil {
		t.Fatal("decimal author ID should be rejected")
	}
}
