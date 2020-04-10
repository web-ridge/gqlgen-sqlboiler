// THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.
package main

import (
	"context"
	"database/sql"

	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/web-ridge/gqlgen-sqlboiler/examples/social-network/auth"
	fm "github.com/web-ridge/gqlgen-sqlboiler/examples/social-network/graphql_models"
	. "github.com/web-ridge/gqlgen-sqlboiler/examples/social-network/helpers"
	dm "github.com/web-ridge/gqlgen-sqlboiler/examples/social-network/models"
	"github.com/web-ridge/gqlgen-sqlboiler/helper"
)

type Resolver struct {
	db *sql.DB
}

const inputKey = "input"

func (r *mutationResolver) CreateComment(ctx context.Context, input fm.CommentCreateInput) (*fm.CommentPayload, error) {

	m := CommentCreateInputToBoiler(&input)

	m.UserID = auth.UserIDFromContext(ctx)

	whiteList := CommentCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
		dm.CommentColumns.UserID,
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, CommentPreloadMap, CommentPayloadPreloadLevels.Comment)
	mods = append(mods, dm.CommentWhere.ID.EQ(m.ID))
	mods = append(mods, dm.CommentWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	pM, err := dm.Comments(mods...).One(ctx, r.db)
	return &fm.CommentPayload{
		Comment: CommentToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreateComments(ctx context.Context, input fm.CommentsCreateInput) (*fm.CommentsPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdateComment(ctx context.Context, id string, input fm.CommentUpdateInput) (*fm.CommentPayload, error) {
	m := CommentUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := CommentID(id)
	if _, err := dm.Comments(
		dm.CommentWhere.ID.EQ(dbID),
		dm.CommentWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, CommentPreloadMap, CommentPayloadPreloadLevels.Comment)
	mods = append(mods, dm.CommentWhere.ID.EQ(dbID))
	mods = append(mods, dm.CommentWhere.UserID.EQ(auth.UserIDFromContext(ctx)))

	pM, err := dm.Comments(mods...).One(ctx, r.db)
	return &fm.CommentPayload{
		Comment: CommentToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdateComments(ctx context.Context, filter *fm.CommentFilter, input fm.CommentUpdateInput) (*fm.CommentsUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.CommentWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, CommentFilterToMods(filter)...)

	m := CommentUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.Comments(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.CommentsUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeleteComment(ctx context.Context, id string) (*fm.CommentDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.CommentWhere.ID.EQ(CommentID(id)),
		dm.CommentWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	}
	_, err := dm.Comments(mods...).DeleteAll(ctx, r.db)
	return &fm.CommentDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeleteComments(ctx context.Context, filter *fm.CommentFilter) (*fm.CommentsDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.CommentWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, CommentFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.CommentColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.Comment))

	var IDsToRemove []helper.RemovedID
	if err := dm.Comments(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.Comments(dm.CommentWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.CommentsDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.Comment),
	}, nil
}

func (r *mutationResolver) CreateCommentLike(ctx context.Context, input fm.CommentLikeCreateInput) (*fm.CommentLikePayload, error) {

	m := CommentLikeCreateInputToBoiler(&input)

	m.UserID = auth.UserIDFromContext(ctx)

	whiteList := CommentLikeCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
		dm.CommentLikeColumns.UserID,
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, CommentLikePreloadMap, CommentLikePayloadPreloadLevels.CommentLike)
	mods = append(mods, dm.CommentLikeWhere.ID.EQ(m.ID))
	mods = append(mods, dm.CommentLikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	pM, err := dm.CommentLikes(mods...).One(ctx, r.db)
	return &fm.CommentLikePayload{
		CommentLike: CommentLikeToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreateCommentLikes(ctx context.Context, input fm.CommentLikesCreateInput) (*fm.CommentLikesPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdateCommentLike(ctx context.Context, id string, input fm.CommentLikeUpdateInput) (*fm.CommentLikePayload, error) {
	m := CommentLikeUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := CommentLikeID(id)
	if _, err := dm.CommentLikes(
		dm.CommentLikeWhere.ID.EQ(dbID),
		dm.CommentLikeWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, CommentLikePreloadMap, CommentLikePayloadPreloadLevels.CommentLike)
	mods = append(mods, dm.CommentLikeWhere.ID.EQ(dbID))
	mods = append(mods, dm.CommentLikeWhere.UserID.EQ(auth.UserIDFromContext(ctx)))

	pM, err := dm.CommentLikes(mods...).One(ctx, r.db)
	return &fm.CommentLikePayload{
		CommentLike: CommentLikeToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdateCommentLikes(ctx context.Context, filter *fm.CommentLikeFilter, input fm.CommentLikeUpdateInput) (*fm.CommentLikesUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.CommentLikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, CommentLikeFilterToMods(filter)...)

	m := CommentLikeUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.CommentLikes(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.CommentLikesUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeleteCommentLike(ctx context.Context, id string) (*fm.CommentLikeDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.CommentLikeWhere.ID.EQ(CommentLikeID(id)),
		dm.CommentLikeWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	}
	_, err := dm.CommentLikes(mods...).DeleteAll(ctx, r.db)
	return &fm.CommentLikeDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeleteCommentLikes(ctx context.Context, filter *fm.CommentLikeFilter) (*fm.CommentLikesDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.CommentLikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, CommentLikeFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.CommentLikeColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.CommentLike))

	var IDsToRemove []helper.RemovedID
	if err := dm.CommentLikes(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.CommentLikes(dm.CommentLikeWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.CommentLikesDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.CommentLike),
	}, nil
}

func (r *mutationResolver) CreateFriendship(ctx context.Context, input fm.FriendshipCreateInput) (*fm.FriendshipPayload, error) {

	m := FriendshipCreateInputToBoiler(&input)

	whiteList := FriendshipCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, FriendshipPreloadMap, FriendshipPayloadPreloadLevels.Friendship)
	mods = append(mods, dm.FriendshipWhere.ID.EQ(m.ID))
	pM, err := dm.Friendships(mods...).One(ctx, r.db)
	return &fm.FriendshipPayload{
		Friendship: FriendshipToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreateFriendships(ctx context.Context, input fm.FriendshipsCreateInput) (*fm.FriendshipsPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdateFriendship(ctx context.Context, id string, input fm.FriendshipUpdateInput) (*fm.FriendshipPayload, error) {
	m := FriendshipUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := FriendshipID(id)
	if _, err := dm.Friendships(
		dm.FriendshipWhere.ID.EQ(dbID),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, FriendshipPreloadMap, FriendshipPayloadPreloadLevels.Friendship)
	mods = append(mods, dm.FriendshipWhere.ID.EQ(dbID))

	pM, err := dm.Friendships(mods...).One(ctx, r.db)
	return &fm.FriendshipPayload{
		Friendship: FriendshipToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdateFriendships(ctx context.Context, filter *fm.FriendshipFilter, input fm.FriendshipUpdateInput) (*fm.FriendshipsUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, FriendshipFilterToMods(filter)...)

	m := FriendshipUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.Friendships(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.FriendshipsUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeleteFriendship(ctx context.Context, id string) (*fm.FriendshipDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.FriendshipWhere.ID.EQ(FriendshipID(id)),
	}
	_, err := dm.Friendships(mods...).DeleteAll(ctx, r.db)
	return &fm.FriendshipDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeleteFriendships(ctx context.Context, filter *fm.FriendshipFilter) (*fm.FriendshipsDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, FriendshipFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.FriendshipColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.Friendship))

	var IDsToRemove []helper.RemovedID
	if err := dm.Friendships(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.Friendships(dm.FriendshipWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.FriendshipsDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.Friendship),
	}, nil
}

func (r *mutationResolver) CreateImage(ctx context.Context, input fm.ImageCreateInput) (*fm.ImagePayload, error) {

	m := ImageCreateInputToBoiler(&input)

	whiteList := ImageCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, ImagePreloadMap, ImagePayloadPreloadLevels.Image)
	mods = append(mods, dm.ImageWhere.ID.EQ(m.ID))
	pM, err := dm.Images(mods...).One(ctx, r.db)
	return &fm.ImagePayload{
		Image: ImageToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreateImages(ctx context.Context, input fm.ImagesCreateInput) (*fm.ImagesPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdateImage(ctx context.Context, id string, input fm.ImageUpdateInput) (*fm.ImagePayload, error) {
	m := ImageUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := ImageID(id)
	if _, err := dm.Images(
		dm.ImageWhere.ID.EQ(dbID),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, ImagePreloadMap, ImagePayloadPreloadLevels.Image)
	mods = append(mods, dm.ImageWhere.ID.EQ(dbID))

	pM, err := dm.Images(mods...).One(ctx, r.db)
	return &fm.ImagePayload{
		Image: ImageToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdateImages(ctx context.Context, filter *fm.ImageFilter, input fm.ImageUpdateInput) (*fm.ImagesUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, ImageFilterToMods(filter)...)

	m := ImageUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.Images(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.ImagesUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeleteImage(ctx context.Context, id string) (*fm.ImageDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.ImageWhere.ID.EQ(ImageID(id)),
	}
	_, err := dm.Images(mods...).DeleteAll(ctx, r.db)
	return &fm.ImageDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeleteImages(ctx context.Context, filter *fm.ImageFilter) (*fm.ImagesDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, ImageFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.ImageColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.Image))

	var IDsToRemove []helper.RemovedID
	if err := dm.Images(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.Images(dm.ImageWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.ImagesDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.Image),
	}, nil
}

func (r *mutationResolver) CreateImageVariation(ctx context.Context, input fm.ImageVariationCreateInput) (*fm.ImageVariationPayload, error) {

	m := ImageVariationCreateInputToBoiler(&input)

	whiteList := ImageVariationCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, ImageVariationPreloadMap, ImageVariationPayloadPreloadLevels.ImageVariation)
	mods = append(mods, dm.ImageVariationWhere.ID.EQ(m.ID))
	pM, err := dm.ImageVariations(mods...).One(ctx, r.db)
	return &fm.ImageVariationPayload{
		ImageVariation: ImageVariationToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreateImageVariations(ctx context.Context, input fm.ImageVariationsCreateInput) (*fm.ImageVariationsPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdateImageVariation(ctx context.Context, id string, input fm.ImageVariationUpdateInput) (*fm.ImageVariationPayload, error) {
	m := ImageVariationUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := ImageVariationID(id)
	if _, err := dm.ImageVariations(
		dm.ImageVariationWhere.ID.EQ(dbID),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, ImageVariationPreloadMap, ImageVariationPayloadPreloadLevels.ImageVariation)
	mods = append(mods, dm.ImageVariationWhere.ID.EQ(dbID))

	pM, err := dm.ImageVariations(mods...).One(ctx, r.db)
	return &fm.ImageVariationPayload{
		ImageVariation: ImageVariationToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdateImageVariations(ctx context.Context, filter *fm.ImageVariationFilter, input fm.ImageVariationUpdateInput) (*fm.ImageVariationsUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, ImageVariationFilterToMods(filter)...)

	m := ImageVariationUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.ImageVariations(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.ImageVariationsUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeleteImageVariation(ctx context.Context, id string) (*fm.ImageVariationDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.ImageVariationWhere.ID.EQ(ImageVariationID(id)),
	}
	_, err := dm.ImageVariations(mods...).DeleteAll(ctx, r.db)
	return &fm.ImageVariationDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeleteImageVariations(ctx context.Context, filter *fm.ImageVariationFilter) (*fm.ImageVariationsDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, ImageVariationFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.ImageVariationColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.ImageVariation))

	var IDsToRemove []helper.RemovedID
	if err := dm.ImageVariations(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.ImageVariations(dm.ImageVariationWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.ImageVariationsDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.ImageVariation),
	}, nil
}

func (r *mutationResolver) CreateLike(ctx context.Context, input fm.LikeCreateInput) (*fm.LikePayload, error) {

	m := LikeCreateInputToBoiler(&input)

	m.UserID = auth.UserIDFromContext(ctx)

	whiteList := LikeCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
		dm.LikeColumns.UserID,
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, LikePreloadMap, LikePayloadPreloadLevels.Like)
	mods = append(mods, dm.LikeWhere.ID.EQ(m.ID))
	mods = append(mods, dm.LikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	pM, err := dm.Likes(mods...).One(ctx, r.db)
	return &fm.LikePayload{
		Like: LikeToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreateLikes(ctx context.Context, input fm.LikesCreateInput) (*fm.LikesPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdateLike(ctx context.Context, id string, input fm.LikeUpdateInput) (*fm.LikePayload, error) {
	m := LikeUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := LikeID(id)
	if _, err := dm.Likes(
		dm.LikeWhere.ID.EQ(dbID),
		dm.LikeWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, LikePreloadMap, LikePayloadPreloadLevels.Like)
	mods = append(mods, dm.LikeWhere.ID.EQ(dbID))
	mods = append(mods, dm.LikeWhere.UserID.EQ(auth.UserIDFromContext(ctx)))

	pM, err := dm.Likes(mods...).One(ctx, r.db)
	return &fm.LikePayload{
		Like: LikeToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdateLikes(ctx context.Context, filter *fm.LikeFilter, input fm.LikeUpdateInput) (*fm.LikesUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.LikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, LikeFilterToMods(filter)...)

	m := LikeUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.Likes(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.LikesUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeleteLike(ctx context.Context, id string) (*fm.LikeDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.LikeWhere.ID.EQ(LikeID(id)),
		dm.LikeWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	}
	_, err := dm.Likes(mods...).DeleteAll(ctx, r.db)
	return &fm.LikeDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeleteLikes(ctx context.Context, filter *fm.LikeFilter) (*fm.LikesDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.LikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, LikeFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.LikeColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.Like))

	var IDsToRemove []helper.RemovedID
	if err := dm.Likes(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.Likes(dm.LikeWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.LikesDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.Like),
	}, nil
}

func (r *mutationResolver) CreatePost(ctx context.Context, input fm.PostCreateInput) (*fm.PostPayload, error) {

	m := PostCreateInputToBoiler(&input)

	m.UserID = auth.UserIDFromContext(ctx)

	whiteList := PostCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
		dm.PostColumns.UserID,
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, PostPreloadMap, PostPayloadPreloadLevels.Post)
	mods = append(mods, dm.PostWhere.ID.EQ(m.ID))
	mods = append(mods, dm.PostWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	pM, err := dm.Posts(mods...).One(ctx, r.db)
	return &fm.PostPayload{
		Post: PostToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreatePosts(ctx context.Context, input fm.PostsCreateInput) (*fm.PostsPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdatePost(ctx context.Context, id string, input fm.PostUpdateInput) (*fm.PostPayload, error) {
	m := PostUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := PostID(id)
	if _, err := dm.Posts(
		dm.PostWhere.ID.EQ(dbID),
		dm.PostWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, PostPreloadMap, PostPayloadPreloadLevels.Post)
	mods = append(mods, dm.PostWhere.ID.EQ(dbID))
	mods = append(mods, dm.PostWhere.UserID.EQ(auth.UserIDFromContext(ctx)))

	pM, err := dm.Posts(mods...).One(ctx, r.db)
	return &fm.PostPayload{
		Post: PostToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdatePosts(ctx context.Context, filter *fm.PostFilter, input fm.PostUpdateInput) (*fm.PostsUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.PostWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, PostFilterToMods(filter)...)

	m := PostUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.Posts(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.PostsUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeletePost(ctx context.Context, id string) (*fm.PostDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.PostWhere.ID.EQ(PostID(id)),
		dm.PostWhere.UserID.EQ(auth.UserIDFromContext(ctx)),
	}
	_, err := dm.Posts(mods...).DeleteAll(ctx, r.db)
	return &fm.PostDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeletePosts(ctx context.Context, filter *fm.PostFilter) (*fm.PostsDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, dm.PostWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, PostFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.PostColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.Post))

	var IDsToRemove []helper.RemovedID
	if err := dm.Posts(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.Posts(dm.PostWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.PostsDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.Post),
	}, nil
}

func (r *mutationResolver) CreateUser(ctx context.Context, input fm.UserCreateInput) (*fm.UserPayload, error) {

	m := UserCreateInputToBoiler(&input)

	whiteList := UserCreateInputToBoilerWhitelist(
		helper.GetInputFromContext(ctx, inputKey),
	)
	if err := m.Insert(ctx, r.db, whiteList); err != nil {
		return nil, err
	}

	// resolve requested fields after creating
	mods := helper.GetPreloadModsWithLevel(ctx, UserPreloadMap, UserPayloadPreloadLevels.User)
	mods = append(mods, dm.UserWhere.ID.EQ(m.ID))
	pM, err := dm.Users(mods...).One(ctx, r.db)
	return &fm.UserPayload{
		User: UserToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) CreateUsers(ctx context.Context, input fm.UsersCreateInput) (*fm.UsersPayload, error) {
	// TODO: Implement batch create
	return nil, nil
}

func (r *mutationResolver) UpdateUser(ctx context.Context, id string, input fm.UserUpdateInput) (*fm.UserPayload, error) {
	m := UserUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)

	dbID := UserID(id)
	if _, err := dm.Users(
		dm.UserWhere.ID.EQ(dbID),
	).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	// resolve requested fields after updating
	mods := helper.GetPreloadModsWithLevel(ctx, UserPreloadMap, UserPayloadPreloadLevels.User)
	mods = append(mods, dm.UserWhere.ID.EQ(dbID))

	pM, err := dm.Users(mods...).One(ctx, r.db)
	return &fm.UserPayload{
		User: UserToGraphQL(pM, nil),
	}, err
}

func (r *mutationResolver) UpdateUsers(ctx context.Context, filter *fm.UserFilter, input fm.UserUpdateInput) (*fm.UsersUpdatePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, UserFilterToMods(filter)...)

	m := UserUpdateInputToModelM(helper.GetInputFromContext(ctx, inputKey), input)
	if _, err := dm.Users(mods...).UpdateAll(ctx, r.db, m); err != nil {
		return nil, err
	}

	return &fm.UsersUpdatePayload{
		Ok: true,
	}, nil
}

func (r *mutationResolver) DeleteUser(ctx context.Context, id string) (*fm.UserDeletePayload, error) {
	mods := []qm.QueryMod{
		dm.UserWhere.ID.EQ(UserID(id)),
	}
	_, err := dm.Users(mods...).DeleteAll(ctx, r.db)
	return &fm.UserDeletePayload{
		ID: id,
	}, err
}

func (r *mutationResolver) DeleteUsers(ctx context.Context, filter *fm.UserFilter) (*fm.UsersDeletePayload, error) {
	var mods []qm.QueryMod
	mods = append(mods, UserFilterToMods(filter)...)
	mods = append(mods, qm.Select(dm.UserColumns.ID))
	mods = append(mods, qm.From(dm.TableNames.User))

	var IDsToRemove []helper.RemovedID
	if err := dm.Users(mods...).Bind(ctx, r.db, IDsToRemove); err != nil {
		return nil, err
	}

	boilerIDs := helper.RemovedIDsToUint(IDsToRemove)
	if _, err := dm.Users(dm.UserWhere.ID.IN(boilerIDs)).DeleteAll(ctx, r.db); err != nil {
		return nil, err
	}

	return &fm.UsersDeletePayload{
		Ids: helper.IDsToGraphQL(boilerIDs, dm.TableNames.User),
	}, nil
}

func (r *queryResolver) Comment(ctx context.Context, id string) (*fm.Comment, error) {
	dbID := CommentID(id)
	mods := helper.GetPreloadMods(ctx, CommentPreloadMap)
	mods = append(mods, dm.CommentWhere.ID.EQ(dbID))
	mods = append(mods, dm.CommentWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	m, err := dm.Comments(mods...).One(ctx, r.db)
	return CommentToGraphQL(m, nil), err
}

func (r *queryResolver) Comments(ctx context.Context, filter *fm.CommentFilter) ([]*fm.Comment, error) {
	mods := helper.GetPreloadMods(ctx, CommentPreloadMap)
	mods = append(mods, dm.CommentWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, CommentFilterToMods(filter)...)
	a, err := dm.Comments(mods...).All(ctx, r.db)
	return CommentsToGraphQL(a, nil), err
}

func (r *queryResolver) CommentLike(ctx context.Context, id string) (*fm.CommentLike, error) {
	dbID := CommentLikeID(id)
	mods := helper.GetPreloadMods(ctx, CommentLikePreloadMap)
	mods = append(mods, dm.CommentLikeWhere.ID.EQ(dbID))
	mods = append(mods, dm.CommentLikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	m, err := dm.CommentLikes(mods...).One(ctx, r.db)
	return CommentLikeToGraphQL(m, nil), err
}

func (r *queryResolver) CommentLikes(ctx context.Context, filter *fm.CommentLikeFilter) ([]*fm.CommentLike, error) {
	mods := helper.GetPreloadMods(ctx, CommentLikePreloadMap)
	mods = append(mods, dm.CommentLikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, CommentLikeFilterToMods(filter)...)
	a, err := dm.CommentLikes(mods...).All(ctx, r.db)
	return CommentLikesToGraphQL(a, nil), err
}

func (r *queryResolver) Friendship(ctx context.Context, id string) (*fm.Friendship, error) {
	dbID := FriendshipID(id)
	mods := helper.GetPreloadMods(ctx, FriendshipPreloadMap)
	mods = append(mods, dm.FriendshipWhere.ID.EQ(dbID))
	m, err := dm.Friendships(mods...).One(ctx, r.db)
	return FriendshipToGraphQL(m, nil), err
}

func (r *queryResolver) Friendships(ctx context.Context, filter *fm.FriendshipFilter) ([]*fm.Friendship, error) {
	mods := helper.GetPreloadMods(ctx, FriendshipPreloadMap)
	mods = append(mods, FriendshipFilterToMods(filter)...)
	a, err := dm.Friendships(mods...).All(ctx, r.db)
	return FriendshipsToGraphQL(a, nil), err
}

func (r *queryResolver) Image(ctx context.Context, id string) (*fm.Image, error) {
	dbID := ImageID(id)
	mods := helper.GetPreloadMods(ctx, ImagePreloadMap)
	mods = append(mods, dm.ImageWhere.ID.EQ(dbID))
	m, err := dm.Images(mods...).One(ctx, r.db)
	return ImageToGraphQL(m, nil), err
}

func (r *queryResolver) Images(ctx context.Context, filter *fm.ImageFilter) ([]*fm.Image, error) {
	mods := helper.GetPreloadMods(ctx, ImagePreloadMap)
	mods = append(mods, ImageFilterToMods(filter)...)
	a, err := dm.Images(mods...).All(ctx, r.db)
	return ImagesToGraphQL(a, nil), err
}

func (r *queryResolver) ImageVariation(ctx context.Context, id string) (*fm.ImageVariation, error) {
	dbID := ImageVariationID(id)
	mods := helper.GetPreloadMods(ctx, ImageVariationPreloadMap)
	mods = append(mods, dm.ImageVariationWhere.ID.EQ(dbID))
	m, err := dm.ImageVariations(mods...).One(ctx, r.db)
	return ImageVariationToGraphQL(m, nil), err
}

func (r *queryResolver) ImageVariations(ctx context.Context, filter *fm.ImageVariationFilter) ([]*fm.ImageVariation, error) {
	mods := helper.GetPreloadMods(ctx, ImageVariationPreloadMap)
	mods = append(mods, ImageVariationFilterToMods(filter)...)
	a, err := dm.ImageVariations(mods...).All(ctx, r.db)
	return ImageVariationsToGraphQL(a, nil), err
}

func (r *queryResolver) Like(ctx context.Context, id string) (*fm.Like, error) {
	dbID := LikeID(id)
	mods := helper.GetPreloadMods(ctx, LikePreloadMap)
	mods = append(mods, dm.LikeWhere.ID.EQ(dbID))
	mods = append(mods, dm.LikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	m, err := dm.Likes(mods...).One(ctx, r.db)
	return LikeToGraphQL(m, nil), err
}

func (r *queryResolver) Likes(ctx context.Context, filter *fm.LikeFilter) ([]*fm.Like, error) {
	mods := helper.GetPreloadMods(ctx, LikePreloadMap)
	mods = append(mods, dm.LikeWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, LikeFilterToMods(filter)...)
	a, err := dm.Likes(mods...).All(ctx, r.db)
	return LikesToGraphQL(a, nil), err
}

func (r *queryResolver) Post(ctx context.Context, id string) (*fm.Post, error) {
	dbID := PostID(id)
	mods := helper.GetPreloadMods(ctx, PostPreloadMap)
	mods = append(mods, dm.PostWhere.ID.EQ(dbID))
	mods = append(mods, dm.PostWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	m, err := dm.Posts(mods...).One(ctx, r.db)
	return PostToGraphQL(m, nil), err
}

func (r *queryResolver) Posts(ctx context.Context, filter *fm.PostFilter) ([]*fm.Post, error) {
	mods := helper.GetPreloadMods(ctx, PostPreloadMap)
	mods = append(mods, dm.PostWhere.UserID.EQ(
		auth.UserIDFromContext(ctx),
	))
	mods = append(mods, PostFilterToMods(filter)...)
	a, err := dm.Posts(mods...).All(ctx, r.db)
	return PostsToGraphQL(a, nil), err
}

func (r *queryResolver) User(ctx context.Context, id string) (*fm.User, error) {
	dbID := UserID(id)
	mods := helper.GetPreloadMods(ctx, UserPreloadMap)
	mods = append(mods, dm.UserWhere.ID.EQ(dbID))
	m, err := dm.Users(mods...).One(ctx, r.db)
	return UserToGraphQL(m, nil), err
}

func (r *queryResolver) Users(ctx context.Context, filter *fm.UserFilter) ([]*fm.User, error) {
	mods := helper.GetPreloadMods(ctx, UserPreloadMap)
	mods = append(mods, UserFilterToMods(filter)...)
	a, err := dm.Users(mods...).All(ctx, r.db)
	return UsersToGraphQL(a, nil), err
}

func (r *Resolver) Mutation() fm.MutationResolver { return &mutationResolver{r} }
func (r *Resolver) Query() fm.QueryResolver       { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
