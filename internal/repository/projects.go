package repository

import (
	"github.com/google/uuid"
	"github.com/iantal/rm/internal/domain"
	"github.com/iantal/rm/internal/util"
	"github.com/jinzhu/gorm"
)

// ProjectDB defines the CRUD operations for storing projects in the db
type ProjectDB struct {
	log *util.StandardLogger
	db  *gorm.DB
}

// NewProjectDB returns a ProjectDB object for handling CRUD operations
func NewProjectDB(log *util.StandardLogger, db *gorm.DB) *ProjectDB {
	db.AutoMigrate(&domain.Project{})
	return &ProjectDB{
		log: log,
		db:  db,
	}
}

// AddProject adds a project to the db
func (p *ProjectDB) AddProject(project *domain.Project) {
	p.db.Create(&project)
	return
}

func (p *ProjectDB) UpdateProject(project *domain.Project) {
	ep := &domain.Project{}
	p.db.Find(&ep, "project_id = ? and commit_hash", project.ProjectID, project.CommitHash)

	if ep != nil {
		p.db.Save(&ep)
	} else {
		p.AddProject(project)
	}
}

// GetProjects returns all existing projects in the db
func (p *ProjectDB) GetProjects() ([]*domain.Project, error) {
	var projects []*domain.Project
	p.db.Find(&projects)
	return projects, nil
}

// GetProjectByID returns the project with the given id
func (p *ProjectDB) GetProjectByID(id string) *domain.Project {
	project := &domain.Project{}
	uid, err := uuid.Parse(id)
	if err != nil {
		p.log.Error("Project with projectId {} was not found")
		return nil
	}
	p.db.Find(&project, "project_id = ?", uid)
	return project
}

// GetProjectByIDAndCommit returns the project with the given id and commit
func (p *ProjectDB) GetProjectByIDAndCommit(id, commit string) *domain.Project {
	project := &domain.Project{}
	uid, err := uuid.Parse(id)
	if err != nil {
		p.log.Error("Project with projectId {} was not found")
		return nil
	}
	p.db.Where("project_id = ? AND commit_hash = ?", uid, commit).Find(&project)
	return project
}
