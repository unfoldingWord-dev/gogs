// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(repo *models.Repository, mode models.AccessMode) *api.Repository {
	return innerToRepo(repo, mode, false)
}

func innerToRepo(repo *models.Repository, mode models.AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: mode >= models.AccessModeAdmin,
		Push:  mode >= models.AccessModeWrite,
		Pull:  mode >= models.AccessModeRead,
	}
	if !isParent {
		err := repo.GetBaseRepo()
		if err != nil {
			return nil
		}
		if repo.BaseRepo != nil {
			parent = innerToRepo(repo.BaseRepo, mode, true)
		}
	}

	//check enabled/disabled units
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.GetUnit(models.UnitTypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(models.UnitTypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    config.ExternalTrackerURL,
			ExternalTrackerFormat: config.ExternalTrackerFormat,
			ExternalTrackerStyle:  config.ExternalTrackerStyle,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(models.UnitTypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(models.UnitTypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
	}
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	if unit, err := repo.GetUnit(models.UnitTypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
	}
	hasProjects := false
	if _, err := repo.GetUnit(models.UnitTypeProjects); err == nil {
		hasProjects = true
	}

	if err := repo.GetOwner(); err != nil {
		return nil
	}

	numReleases, _ := models.GetReleaseCountByRepoID(repo.ID, models.FindReleasesOptions{IncludeDrafts: false, IncludeTags: true})

	/* DCS Customizations */
	var catalog *api.Catalog
	if latestReleaseMetadata, err := models.GetLatestCatalogMetadataByRepoID(repo.ID, false); err != nil && !models.IsErrDoor43MetadataNotExist(err) {
		log.Error("GetLatestCatalogMetadataByRepoID: %v", err)
	} else if latestReleaseMetadata != nil {
		r := latestReleaseMetadata.Release
		catalog = &api.Catalog{
			Release: &api.Release{
				ID:           r.ID,
				TagName:      r.TagName,
				Target:       r.Target,
				Title:        r.Title,
				Note:         r.Note,
				URL:          r.APIURL(),
				HTMLURL:      r.HTMLURL(),
				TarURL:       r.TarURL(),
				ZipURL:       r.ZipURL(),
				IsDraft:      r.IsDraft,
				IsPrerelease: r.IsPrerelease,
				CreatedAt:    r.CreatedUnix.AsTime(),
				PublishedAt:  r.CreatedUnix.AsTime(),
				Publisher:    &api.User{
					ID:        r.Publisher.ID,
					UserName:  r.Publisher.Name,
					FullName:  markup.Sanitize(r.Publisher.FullName),
					Email:     r.Publisher.GetEmail(),
					AvatarURL: r.Publisher.AvatarLink(),
					Created:   r.Publisher.CreatedUnix.AsTime(),
					RepoLanguages: r.Publisher.GetRepoLanguages(),
				},
			},
		}
	}

	metadata, err := models.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, 0)
	if err != nil && !models.IsErrDoor43MetadataNotExist(err) {
		log.Error("GetDoor43MetadataByRepoIDAndReleaseID: %v", err)
	}
	if metadata == nil {
		metadata, err = repo.GetLatestPreProdCatalogMetadata()
		if err != nil {
			log.Error("GetLatestPreProdCatalogMetadata: %v", err)
		}
	}

	var language, title, subject, checkingLevel string
	var books []string
	if metadata != nil {
		language = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
		title = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["title"].(string)
		subject = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string)
		books = metadata.GetBooks()
		checkingLevel = (*metadata.Metadata)["checking"].(map[string]interface{})["checking_level"].(string)
	}
	/* END DCS Customizations */

	return &api.Repository{
		ID:                        repo.ID,
		Owner:                     ToUser(repo.Owner, mode != models.AccessModeNone, mode >= models.AccessModeAdmin),
		Name:                      repo.Name,
		FullName:                  repo.FullName(),
		Description:               repo.Description,
		Private:                   repo.IsPrivate,
		Template:                  repo.IsTemplate,
		Empty:                     repo.IsEmpty,
		Archived:                  repo.IsArchived,
		Size:                      int(repo.Size / 1024),
		Fork:                      repo.IsFork,
		Parent:                    parent,
		Mirror:                    repo.IsMirror,
		HTMLURL:                   repo.HTMLURL(),
		SSHURL:                    cloneLink.SSH,
		CloneURL:                  cloneLink.HTTPS,
		Website:                   repo.Website,
		Stars:                     repo.NumStars,
		Forks:                     repo.NumForks,
		Watchers:                  repo.NumWatches,
		OpenIssues:                repo.NumOpenIssues,
		OpenPulls:                 repo.NumOpenPulls,
		Releases:                  int(numReleases),
		DefaultBranch:             repo.DefaultBranch,
		Created:                   repo.CreatedUnix.AsTime(),
		Updated:                   repo.UpdatedUnix.AsTime(),
		Permissions:               permission,
		HasIssues:                 hasIssues,
		ExternalTracker:           externalTracker,
		InternalTracker:           internalTracker,
		HasWiki:                   hasWiki,
		HasProjects:               hasProjects,
		ExternalWiki:              externalWiki,
		HasPullRequests:           hasPullRequests,
		IgnoreWhitespaceConflicts: ignoreWhitespaceConflicts,
		AllowMerge:                allowMerge,
		AllowRebase:               allowRebase,
		AllowRebaseMerge:          allowRebaseMerge,
		AllowSquash:               allowSquash,
		AvatarURL:                 repo.AvatarLink(),
		Language:                  language,
		Title:                     title,
		Subject:                   subject,
		Books:                     books,
		CheckingLevel:             checkingLevel,
		Catalog:                   catalog,
		Internal:                  !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
	}
}
