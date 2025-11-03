package models

type Device struct {
	ID						string	`json:"id"`
	Name					string	`json:"name"`
	MAC						string	`json:"mac"`
	Status				string	`json:"status"`
	IP						string 	`json:"ip,omitempty"`
	PingEnabled		bool	 	`json:"ping_enabled"`
	LastSeen			int64		`json:"last_seen"`
}
