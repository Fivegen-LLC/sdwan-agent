package entities

const InstallUpdateManagerScript = `
#!/bin/bash

new_version=$1
current_version=$2

new_deb_path="/var/cache/apt/archives/sdwan-update-manager_${new_version}_amd64.deb"
current_deb_path="/var/cache/apt/archives/sdwan-update-manager_${current_version}_amd64.deb"

if [ ! -f "$new_deb_path" ]; then
	echo "File not found: $new_deb_path" >&2
	exit 1
fi

# install new version
dpkg -i --force-confold "$new_deb_path"

sleep 5

# check daemon state
active_state=$(systemctl show --no-pager sdwan-update-manager | grep "ActiveState" | awk -F= '{print $2}')

if [ "$active_state" != "active" ]; then
	# install old version
    dpkg -i --force-confold "$current_deb_path"
fi
`

type (
	InstallPackageRequest struct {
		PackagesToInstall PackageItems `json:"packages"`
	}

	DownloadPackageRequest struct {
		PackagesToDownload PackageItems `json:"packages"`
	}

	PackageItem struct {
		Name            string `json:"name" validate:"required"`
		Version         string `json:"version" validate:"required"`
		PreviousVersion string `json:"previousVersion"`
		IsNew           bool   `json:"isNew"`
	}

	PackageItems []PackageItem

	ActualPackageVersion struct {
		Name            string `json:"name"`
		Version         string `json:"version"`
		NewIsDownloaded bool   `json:"newIsDownloaded"`
	}

	ActualPackageVersions []ActualPackageVersion
)
