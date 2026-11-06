# Maintainer: Egor0jdk <https://github.com/Egor0jdk>
pkgname=monocfg-git
pkgver=r4.gbd59fdd
pkgrel=1
pkgdesc="Declarative configuration and package manager for Linux with native Bedrock Linux support"
arch=('x86_64' 'aarch64')
url="https://github.com/Egor0jdk/monocfg"
license=('GPL-3.0-only')
depends=('glibc')
makedepends=('go' 'git')
provides=('monocfg')
conflicts=('monocfg')
options=(!debug)
source=("git+${url}.git")
md5sums=('SKIP')

pkgver() {
  cd "${srcdir}/${pkgname%-git}"
  printf "r%s.g%s" "$(git rev-list --count HEAD)" "$(git rev-parse --short HEAD)"
}

build() {
  cd "${srcdir}/${pkgname%-git}"
  export CGO_ENABLED=0
  go build -o monocfg -ldflags="-s -w" main.go
}

package() {
  cd "${srcdir}/${pkgname%-git}"
  install -Dm755 monocfg "${pkgdir}/usr/bin/monocfg"
}
