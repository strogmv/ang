package gamepoc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// checkLoadingImages fails the build when a loading screen names a picture that is not in the Unity Resources folder,
// so a content typo is caught at generation time and not as a black screen in the game.
func checkLoadingImages(unity string, loading Loading) error {
	ids := make([]string, 0, len(loading.Screens))
	for id := range loading.Screens {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := checkPicture(unity, "loading screen "+id, loading.Screens[id].Image); err != nil {
			return err
		}
	}
	return nil
}

// checkMenuImage does the same for the main-menu backdrop; an empty world.menu is allowed (flat menu).
func checkMenuImage(unity string, menu Menu) error {
	if menu.Background == "" {
		return nil
	}
	return checkPicture(unity, "main menu", menu.Background)
}

func checkPicture(unity, owner, image string) error {
	for _, ext := range []string{".jpg", ".png"} {
		if info, err := os.Stat(filepath.Join(unity, "Assets/Resources", image+ext)); err == nil && !info.IsDir() {
			return nil
		}
	}
	return fmt.Errorf("%s: no picture Assets/Resources/%s.jpg or .png", owner, image)
}
