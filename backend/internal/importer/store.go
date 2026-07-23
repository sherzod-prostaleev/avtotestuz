package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/db/sqlc"
)

type StoreOptions struct {
	MarkVerified bool        // licensed/trusted import → translations verified
	Images       ImageSource // resolves relative image paths
	Source       string      // provenance label stored on rows
}

// Store validates and persists the dataset in one transaction.
// Quarantined questions are stored (excluded from serving); broken variants are skipped.
func Store(ctx context.Context, pool *pgxpool.Pool, blobs blob.Store, ds Dataset, opts StoreOptions) (Report, error) {
	issues := Validate(ds)
	quarantined := QuarantinedIDs(issues)
	brokenVariants := BrokenVariants(issues)
	rep := Report{Issues: issues}

	status := "pending"
	if opts.MarkVerified {
		status = "verified"
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return rep, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	imageIDs := map[string]uuid.UUID{} // rel path → image id
	putImage := func(rel string) (uuid.NullUUID, error) {
		if rel == "" {
			return uuid.NullUUID{}, nil
		}
		if id, ok := imageIDs[rel]; ok {
			return uuid.NullUUID{UUID: id, Valid: true}, nil
		}
		data, err := opts.Images.Open(rel)
		if err != nil {
			return uuid.NullUUID{}, fmt.Errorf("image %s: %w", rel, err)
		}
		sum := sha256.Sum256(data)
		hexSum := hex.EncodeToString(sum[:])
		ext := path.Ext(rel)
		key := "images/" + hexSum + ext
		mime := "image/png"
		switch ext {
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".webp":
			mime = "image/webp"
		case ".svg":
			mime = "image/svg+xml"
		}
		if err := blobs.Put(ctx, key, mime, data); err != nil {
			return uuid.NullUUID{}, fmt.Errorf("blob put %s: %w", key, err)
		}
		id, err := q.UpsertImage(ctx, sqlc.UpsertImageParams{
			StorageKey: key, Sha256: hexSum, Mime: mime, Source: opts.Source,
		})
		if err != nil {
			return uuid.NullUUID{}, fmt.Errorf("upsert image: %w", err)
		}
		imageIDs[rel] = id
		rep.Images++
		return uuid.NullUUID{UUID: id, Valid: true}, nil
	}

	categoryIDs := map[string]uuid.UUID{}
	for _, c := range ds.Categories {
		id, err := q.UpsertCategory(ctx, sqlc.UpsertCategoryParams{Code: c.Code, SortOrder: int32(c.Sort)})
		if err != nil {
			return rep, fmt.Errorf("category %s: %w", c.Code, err)
		}
		categoryIDs[c.Code] = id
		rep.Categories++
		for loc, name := range c.Names {
			if err := q.UpsertCategoryTranslation(ctx, sqlc.UpsertCategoryTranslationParams{
				CategoryID: id, Locale: loc, Name: name, Status: status,
			}); err != nil {
				return rep, err
			}
		}
	}

	groupIDs := map[string]uuid.UUID{}
	for _, g := range ds.SignGroups {
		id, err := q.UpsertSignGroup(ctx, sqlc.UpsertSignGroupParams{Code: g.Code, SortOrder: int32(g.Sort)})
		if err != nil {
			return rep, fmt.Errorf("sign group %s: %w", g.Code, err)
		}
		groupIDs[g.Code] = id
		for loc, name := range g.Names {
			if err := q.UpsertSignGroupTranslation(ctx, sqlc.UpsertSignGroupTranslationParams{
				SignGroupID: id, Locale: loc, Name: name, Status: status,
			}); err != nil {
				return rep, err
			}
		}
	}

	signIDs := map[string]uuid.UUID{}
	for _, s := range ds.Signs {
		gid, ok := groupIDs[s.Group]
		if !ok {
			rep.Issues = append(rep.Issues, Issue{Entity: "sign", ID: s.Code, Code: "unknown_sign_group", Detail: s.Group})
			continue
		}
		imgID, err := putImage(s.Image)
		if err != nil {
			return rep, err
		}
		id, err := q.UpsertSign(ctx, sqlc.UpsertSignParams{
			GroupID: gid, Code: s.Code, ImageID: imgID, SortOrder: int32(s.Sort),
		})
		if err != nil {
			return rep, fmt.Errorf("sign %s: %w", s.Code, err)
		}
		signIDs[s.Code] = id
		rep.Signs++
		for loc, name := range s.Names {
			if err := q.UpsertSignTranslation(ctx, sqlc.UpsertSignTranslationParams{
				SignID: id, Locale: loc, Name: name,
				Description: s.Descriptions[loc], Status: status,
			}); err != nil {
				return rep, err
			}
		}
	}

	questionIDs := map[string]uuid.UUID{}
	for _, cq := range ds.Questions {
		catID, ok := categoryIDs[cq.Category]
		if !ok {
			// Unknown category: cannot satisfy NOT NULL FK — skip storing entirely.
			// The unknown_category issue is already recorded by Validate.
			continue
		}
		vstatus := "valid"
		if quarantined[cq.ExtID] {
			vstatus = "quarantined"
		}
		imgID, err := putImage(cq.Image)
		if err != nil {
			return rep, err
		}
		qid, err := q.UpsertQuestion(ctx, sqlc.UpsertQuestionParams{
			SourceExtID: cq.ExtID, CategoryID: catID, ImageID: imgID,
			ContentHash: ContentHash(cq), Source: opts.Source, ValidationStatus: vstatus,
		})
		if err != nil {
			return rep, fmt.Errorf("question %s: %w", cq.ExtID, err)
		}
		questionIDs[cq.ExtID] = qid
		if vstatus == "valid" {
			rep.QuestionsValid++
		} else {
			rep.QuestionsQuarantined++
		}

		if err := q.DeleteAnswersForQuestion(ctx, qid); err != nil {
			return rep, err
		}
		for _, a := range cq.Answers {
			if a.Position < 1 || a.Position > 5 {
				continue // invariant issue already recorded; DB CHECK would reject it
			}
			aImgID, err := putImage(a.Image)
			if err != nil {
				return rep, err
			}
			aid, err := q.InsertAnswer(ctx, sqlc.InsertAnswerParams{
				QuestionID: qid, Position: int16(a.Position), IsCorrect: a.Correct, ImageID: aImgID,
			})
			if err != nil {
				return rep, fmt.Errorf("answer %s/%d: %w", cq.ExtID, a.Position, err)
			}
			if a.Correct {
				if err := q.SetCorrectAnswer(ctx, sqlc.SetCorrectAnswerParams{
					ID: qid, CorrectAnswerID: uuid.NullUUID{UUID: aid, Valid: true},
				}); err != nil {
					return rep, err
				}
			}
			for loc, text := range a.Texts {
				if err := q.InsertAnswerTranslation(ctx, sqlc.InsertAnswerTranslationParams{
					AnswerID: aid, Locale: loc, Text: text, Status: status,
				}); err != nil {
					return rep, err
				}
			}
		}
		for loc, text := range cq.Texts {
			if err := q.UpsertQuestionTranslation(ctx, sqlc.UpsertQuestionTranslationParams{
				QuestionID: qid, Locale: loc, Text: text, Status: status, Source: opts.Source,
			}); err != nil {
				return rep, err
			}
		}
		if err := q.DeleteQuestionSigns(ctx, qid); err != nil {
			return rep, err
		}
		for _, sc := range cq.Signs {
			sid, ok := signIDs[sc]
			if !ok {
				continue // unknown_sign issue already recorded
			}
			if err := q.InsertQuestionSign(ctx, sqlc.InsertQuestionSignParams{QuestionID: qid, SignID: sid}); err != nil {
				return rep, err
			}
		}
	}

	for _, v := range ds.Variants {
		if brokenVariants[fmt.Sprintf("%d", v.Number)] {
			rep.VariantsSkipped++
			continue
		}
		vid, err := q.UpsertVariant(ctx, sqlc.UpsertVariantParams{Number: int32(v.Number), SortOrder: int32(v.Number)})
		if err != nil {
			return rep, fmt.Errorf("variant %d: %w", v.Number, err)
		}
		if err := q.DeleteVariantQuestions(ctx, vid); err != nil {
			return rep, err
		}
		for pos, ext := range v.Questions {
			if err := q.InsertVariantQuestion(ctx, sqlc.InsertVariantQuestionParams{
				VariantID: vid, QuestionID: questionIDs[ext], Position: int16(pos + 1),
			}); err != nil {
				return rep, fmt.Errorf("variant %d pos %d: %w", v.Number, pos+1, err)
			}
		}
		rep.VariantsStored++
	}

	for _, e := range ds.Explanations {
		qid, ok := questionIDs[e.Question]
		if !ok {
			rep.Issues = append(rep.Issues, Issue{Entity: "dataset", ID: e.Question, Code: "explanation_unknown_question"})
			continue
		}
		refs, err := json.Marshal(e.LegalRefs)
		if err != nil {
			return rep, err
		}
		eid, err := q.UpsertExplanation(ctx, sqlc.UpsertExplanationParams{QuestionID: qid, LegalRefs: refs})
		if err != nil {
			return rep, err
		}
		for loc, blocks := range e.Blocks {
			bj, err := json.Marshal(blocks)
			if err != nil {
				return rep, err
			}
			st := "draft"
			if opts.MarkVerified {
				st = "verified"
			}
			if err := q.UpsertExplanationTranslation(ctx, sqlc.UpsertExplanationTranslationParams{
				ExplanationID: eid, Locale: loc, Blocks: bj, Status: st, Source: opts.Source,
			}); err != nil {
				return rep, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return rep, fmt.Errorf("commit: %w", err)
	}
	return rep, nil
}
