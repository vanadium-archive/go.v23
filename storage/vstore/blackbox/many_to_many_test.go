package blackbox

// import (
// 	"runtime"
// 	"testing"

// 	"veyron2/rt"
// 	"veyron2/storage"
// 	"veyron2/storage/vstore"
// 	"veyron2/vom"
// )

// func init() {
// 	vom.Register(&Person{})
// 	vom.Register(&Player{})
// 	vom.Register(&Team{})
// 	vom.Register(&Role{})
// 	vom.Register(&DirectPlayer{})
// 	vom.Register(&DirectTeam{})
// }

// // This schema uses a Role relation to indicate who plays for what team.
// // There are also indexes; each Player and each Team have a list of Roles/
// //
// //     Person : a person.
// //     Player : belongs to many teams.
// //     Team : contains many players.
// //     Role : (Player, Team) pair.
// //
// //  / : Dir
// //  /People : Dir
// //  /People/John : Person
// //  /Players : Dir
// //  /Players/John : Player
// //  /Teams : Dir
// //  /Teams/Rockets : Team

// // Person is a person.
// type Person struct {
// 	FullName string
// 	SSN      int
// }

// // Player is a person who has a Role.
// type Player struct {
// 	Person storage.ID
// 	Roles  []storage.ID // Role
// }

// // Team has a set of Roles/
// type Team struct {
// 	Roles []storage.ID // Role
// }

// // Role associates a Player with a Team.
// type Role struct {
// 	Position string
// 	Player   storage.ID // Player
// 	Team     storage.ID
// }

// func newPerson(name string, ssn int) *Person {
// 	return &Person{FullName: name, SSN: ssn}
// }

// func newPlayer(personID storage.ID) *Player {
// 	return &Player{Person: personID}
// }

// func newTeam() *Team {
// 	return &Team{}
// }

// func newRole(pos string, playerID, teamID storage.ID) *Role {
// 	return &Role{Position: pos, Player: playerID, Team: teamID}
// }

// func getPerson(t *testing.T, tx storage.Transaction, name string) (storage.ID, *Person) {
// 	_, file, line, _ := runtime.Caller(1)
// 	e := get(t, tx, name)
// 	v := e.Value
// 	p, ok := v.(*Person)
// 	if !ok {
// 		t.Fatalf("%s(%d): %s: not a Person: %v", file, line, name, v)
// 	}
// 	return e.Stat.ID, p
// }

// func getPlayer(t *testing.T, tx storage.Transaction, name string) (storage.ID, *Player) {
// 	_, file, line, _ := runtime.Caller(1)
// 	e := get(t, tx, name)
// 	v := e.Value
// 	p, ok := v.(*Player)
// 	if !ok {
// 		t.Fatalf("%s(%d): %s: not a Player: %v", file, line, name, v)
// 	}
// 	return e.Stat.ID, p
// }

// func getTeam(t *testing.T, tx storage.Transaction, name string) (storage.ID, *Team) {
// 	_, file, line, _ := runtime.Caller(1)
// 	e := get(t, tx, name)
// 	v := e.Value
// 	p, ok := v.(*Team)
// 	if !ok {
// 		t.Fatalf("%s(%d): %s: not a Team: %v", file, line, name, v)
// 	}
// 	return e.Stat.ID, p
// }

// func getRole(t *testing.T, tx storage.Transaction, name string) (storage.ID, *Role) {
// 	_, file, line, _ := runtime.Caller(1)
// 	e := get(t, tx, name)
// 	v := e.Value
// 	p, ok := v.(*Role)
// 	if !ok {
// 		t.Fatalf("%s(%d): %s: not a Role: %v", file, line, name, v)
// 	}
// 	return e.Stat.ID, p
// }

// func TestManyToManyWithRole(t *testing.T) {
// 	// Note, startServer calls rt.Init().
// 	storeRoot, c := startServer(t)
// 	defer c()
// 	ctx := rt.R().NewContext()
// 	st := vstore.New()

// 	// Create a player John who plays for the Rockets team.
// 	{
// 		tx := st.NewTransaction(ctx, storeRoot)
// 		put(t, tx, "", newDir())
// 		put(t, tx, "People", newDir())
// 		put(t, tx, "Players", newDir())
// 		put(t, tx, "Teams", newDir())

// 		person := newPerson("John", 1234567809)
// 		personID := put(t, tx, "People/John", person)
// 		player := newPlayer(personID)
// 		playerID := put(t, tx, "Players/John", player)
// 		team := newTeam()
// 		teamID := put(t, tx, "Teams/Rockets", team)

// 		// We have to update the team/player to add the cyclic links.
// 		//
// 		// Note: the @ means to append the role to the Roles array.
// 		role := newRole("center", playerID, teamID)
// 		roleID := put(t, tx, "Players/John/Roles/@", role)
// 		put(t, tx, "Teams/Rockets/Roles/@", roleID)

// 		if err := tx.Commit(ctx); err != nil {
// 			t.Fatalf("Unexpected error: %s", err)
// 		}
// 	}

// 	// Verify the state.
// 	{
// 		tx := st.NewTransaction(ctx, storeRoot)
// 		pID, p := getPerson(t, tx, "People/John")
// 		if p.FullName != "John" {
// 			t.Errorf("Expected %q, got %q", "John", p.FullName)
// 		}

// 		plID, pl := getPlayer(t, tx, "Players/John")
// 		if pl.Person != pID {
// 			t.Errorf("Expected %s, got %s", pID, pl.Person)
// 		}

// 		teamID, team := getTeam(t, tx, "Teams/Rockets")
// 		if len(team.Roles) != 1 || len(pl.Roles) != 1 || team.Roles[0] != pl.Roles[0] {
// 			t.Errorf("Expected one role: %v, %v", team, pl)
// 		}

// 		role1ID, role1 := getRole(t, tx, "Players/John/Roles/0")
// 		role2ID, _ := getRole(t, tx, "Teams/Rockets/Roles/0")
// 		if role1ID != role2ID {
// 			t.Errorf("Expected %s, got %s", role1ID, role2ID)
// 		}
// 		if role1.Player != plID {
// 			t.Errorf("Expected %s, got %s", plID, role1.Player)
// 		}
// 		if role1.Team != teamID {
// 			t.Errorf("Expected %s, got %s", teamID, role1.Team)
// 		}
// 	}
// }

// ////////////////////////////////////////////////////////////////////////
// // This schema removes the separate Role object.  Instead the Player refers
// // directly to the Teams, and vice versa.

// // DirectPlayer is a person who plays on a team.
// type DirectPlayer struct {
// 	Person storage.ID
// 	Teams  []storage.ID
// }

// // DirectTeam has a set of players.
// type DirectTeam struct {
// 	Players []storage.ID
// }

// func newDirectPlayer(personID storage.ID) *DirectPlayer {
// 	return &DirectPlayer{Person: personID}
// }

// func newDirectTeam() *DirectTeam {
// 	return &DirectTeam{}
// }

// func getDirectPlayer(t *testing.T, tx storage.Transaction, name string) (storage.ID, *DirectPlayer) {
// 	_, file, line, _ := runtime.Caller(1)
// 	e := get(t, tx, name)
// 	v := e.Value
// 	p, ok := v.(*DirectPlayer)
// 	if !ok {
// 		t.Fatalf("%s(%d): %s: not a DirectPlayer: %v", file, line, name, v)
// 	}
// 	return e.Stat.ID, p
// }

// func getDirectTeam(t *testing.T, tx storage.Transaction, name string) (storage.ID, *DirectTeam) {
// 	_, file, line, _ := runtime.Caller(1)
// 	e := get(t, tx, name)
// 	v := e.Value
// 	p, ok := v.(*DirectTeam)
// 	if !ok {
// 		t.Fatalf("%s(%d): %s: not a DirectTeam: %v", file, line, name, v)
// 	}
// 	return e.Stat.ID, p
// }

// func TestManyToManyDirect(t *testing.T) {
// 	storeRoot, c := startServer(t)
// 	defer c()
// 	ctx := rt.R().NewContext()
// 	st := vstore.New()

// 	// Create a player John who plays for the Rockets team.
// 	{
// 		tx := st.NewTransaction(ctx, storeRoot)
// 		put(t, tx, "", newDir())
// 		put(t, tx, "People", newDir())
// 		put(t, tx, "Players", newDir())
// 		put(t, tx, "Teams", newDir())

// 		person := newPerson("John", 1234567809)
// 		personID := put(t, tx, "People/John", person)
// 		player := newDirectPlayer(personID)
// 		playerID := put(t, tx, "Players/John", player)
// 		team := newDirectTeam()
// 		teamID := put(t, tx, "Teams/Rockets", team)

// 		// We have to update the team/player to add the cyclic links.
// 		put(t, tx, "Players/John/Teams/@", teamID)
// 		put(t, tx, "Teams/Rockets/Players/@", playerID)

// 		if err := tx.Commit(ctx); err != nil {
// 			t.Fatalf("Unexpected error: %s", err)
// 		}
// 	}

// 	// Verify the state.
// 	{
// 		tx := st.NewTransaction(ctx, storeRoot)
// 		pID, p := getPerson(t, tx, "People/John")
// 		plID, pl := getDirectPlayer(t, tx, "Players/John")
// 		teamID, team := getDirectTeam(t, tx, "Teams/Rockets")

// 		if p.FullName != "John" {
// 			t.Errorf("Expected %q, got %q", "John", p.FullName)
// 		}
// 		if pl.Person != pID {
// 			t.Errorf("Expected %s, got %s", pID, pl.Person)
// 		}
// 		if len(pl.Teams) != 1 || pl.Teams[0] != teamID {
// 			t.Errorf("Expected one team: %v, %v", pl.Teams, team.Players)
// 		}
// 		if len(team.Players) != 1 || team.Players[0] != plID {
// 			t.Errorf("Expected one player: %v, %v", team, pl)
// 		}
// 	}
// }
