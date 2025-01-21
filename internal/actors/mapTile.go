package actors

import (
	"cityio/internal/messages"
	"cityio/internal/models"

	"log"

	"github.com/asynkron/protoactor-go/actor"
)

type MapTileActor struct {
	BaseActor
	Tile models.MapTile

	CityPID     *actor.PID
	BuildingPID *actor.PID
	ArmyPIDs    []*actor.PID
	Armies      []models.Army
}

func (state *MapTileActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {

	case messages.CreateMapTileMessage:
		state.Tile = msg.Tile
		state.ArmyPIDs = make([]*actor.PID, 0)
		if !msg.Restore {
			ctx.Send(state.database, messages.CreateMapTileMessage{
				Tile: state.Tile,
			})
		}
		ctx.Respond(messages.CreateMapTileResponseMessage{
			Error: nil,
		})

	case messages.UpdateTileCityPIDMessage:
		state.CityPID = msg.CityPID

	case messages.UpdateTileBuildingPIDMessage:
		state.BuildingPID = msg.BuildingPID

	case messages.AddTileArmyMessage:
		state.ArmyPIDs = append(state.ArmyPIDs, msg.ArmyPID)
		state.Armies = append(state.Armies, msg.Army)
		ctx.Respond(messages.AddTileArmyResponseMessage{
			Error: nil,
		})

	case messages.GetMapTileMessage:
		var city *models.City = nil
		if state.CityPID != nil {
			response, err := Request[messages.GetCityResponseMessage](ctx, state.CityPID, messages.GetCityMessage{})
			if err != nil {
				log.Printf("Error getting city: %s", err)
			} else {
				city = &response.City
			}
		}
		var building *models.Building = nil
		if state.BuildingPID != nil {
			response, err := Request[messages.GetBuildingResponseMessage](ctx, state.BuildingPID, messages.GetBuildingMessage{})
			if err != nil {
				log.Printf("Error getting building: %s", err)
			} else {
				building = &response.Building
			}
		}
		ctx.Respond(messages.GetMapTileResponseMessage{
			Tile:     state.Tile,
			City:     city,
			Building: building,
		})

	case messages.GetMapTileArmiesMessage:
		ctx.Respond(messages.GetMapTileArmiesResponseMessage{
			Armies: state.Armies,
		})
	}
}
