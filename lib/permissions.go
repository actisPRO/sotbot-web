package lib

import "github.com/bwmarrin/discordgo"

const (
	User      int = 0
	Captain   int = 1
	Moderator int = 2
	Admin     int = 3
)

func GetAccessLevelFromRoles(member *discordgo.Member, config Configuration) int {
	for i := 0; i < len(member.Roles); i++ {
		for j := 0; j < len(config.AdminRoles); j++ {
			if member.Roles[i] == config.AdminRoles[j] {
				return Admin
			}
		}
		for j := 0; j < len(config.ModRoles); j++ {
			if member.Roles[i] == config.ModRoles[j] {
				return Moderator
			}
		}
		for j := 0; j < len(config.CaptainRoles); j++ {
			if member.Roles[i] == config.CaptainRoles[j] {
				return Captain
			}
		}
	}

	return 0
}
