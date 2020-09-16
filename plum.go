package goplum

import "log"

func Initialise(plugins []Plugin, configPath string) {
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Unable to read config: %v", err)
	}

	checks := make(map[string]Check)
	for i := range plugins {
		cs := plugins[i].Checks()
		for j := range cs {
			checks[cs[j].Name()] = cs[j]
		}
	}
	log.Printf("Found %d checks from %d plugins\n", len(checks), len(plugins))

	tasks := make([]Task, 0)
	for i := range config.Checks {
		cc := config.Checks[i]
		check, ok := checks[cc.Check]
		if !ok {
			log.Fatalf("Invalid check name in config: %s", cc.Check)
		}

		t, err := check.Create(cc.Params)
		if err != nil {
			log.Fatalf("Unable to create check '%s': %v", cc.Name, err)
		}

		tasks = append(tasks, t)
	}
}
