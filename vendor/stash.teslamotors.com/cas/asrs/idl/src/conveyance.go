package asrs

import "strings"

const TrayIdDelimiter string = ","

func TrayToString(tray *Tray) (string, error) {
	ids := tray.GetTrayId()
	return strings.Join(ids, TrayIdDelimiter), nil
}

func TrayFromString(trayIds string) (*Tray, error) {
	candidates := strings.Split(trayIds, TrayIdDelimiter)
	tray := Tray{
		TrayId: make([]string, 0, len(candidates)),
	}
	for _, id := range candidates {
		if strings.TrimSpace(id) != "" {
			tray.TrayId = append(tray.TrayId, id)
		}
	}

	return &tray, nil
}
