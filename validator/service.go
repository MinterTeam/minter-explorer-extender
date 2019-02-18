package validator

import (
	"github.com/MinterTeam/minter-explorer-extender/helpers"
	"github.com/MinterTeam/minter-explorer-extender/models"
	"github.com/daniildulin/minter-node-api/responses"
)

type Service struct {
	repository *Repository
}

func NewService(r *Repository) *Service {
	return &Service{
		repository: r,
	}
}

//Get validators PK from response and store it to validators table if not exist
func (s *Service) HandleBlockResponse(response *responses.BlockResponse) error {
	var validators []*models.Validator
	for _, v := range response.Result.Validators {
		validators = append(validators, &models.Validator{PublicKey: helpers.RemovePrefix(v.PubKey)})
	}
	return s.repository.SaveAllIfNotExist(validators)
}
