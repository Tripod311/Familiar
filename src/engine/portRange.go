package engine

type PortRange struct {
	MinPort    int
	MaxPort    int
	LastPort   int
	takenPorts []int
}

func NewPortRange(minPort, maxPort int) *PortRange {
	pr := PortRange{
		MinPort:  minPort,
		MaxPort:  maxPort,
		LastPort: minPort,
	}
	return &pr
}

func (pr *PortRange) Acquire() int {
	if len(pr.takenPorts) == pr.MaxPort-pr.MinPort {
		return -1
	}

	for {
		check := pr.LastPort
		taken := false

		for _, used := range pr.takenPorts {
			if check == used {
				taken = true
				break
			}
		}

		result := pr.LastPort
		pr.LastPort++
		if pr.LastPort == pr.MaxPort+1 {
			pr.LastPort = pr.MinPort
		}

		if taken {
			continue
		} else {
			pr.takenPorts = append(pr.takenPorts, pr.LastPort)
			return result
		}
	}
}

func (pr *PortRange) Release(port int) {
	var repl []int

	for _, used := range pr.takenPorts {
		if used != port {
			repl = append(repl, used)
		}
	}

	pr.takenPorts = repl
}
