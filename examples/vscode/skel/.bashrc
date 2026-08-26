# Seeded into the cell's home the first time the container started. It is yours
# from then on: edit it, and a rebuilt image will not touch it.

case $- in
*i*) ;;
*) return ;;
esac

export EDITOR=nano
export PATH="$HOME/.local/bin:$PATH"

HISTSIZE=100000
HISTFILESIZE=200000
HISTCONTROL=ignoreboth
shopt -s histappend checkwinsize

[ -r /usr/share/bash-completion/bash_completion ] && . /usr/share/bash-completion/bash_completion

# nvm is in /opt rather than in the home, because the home is mounted over
# whatever the image put there. Sourcing it is only needed to switch versions:
# the default one is already on PATH.
export NVM_DIR=/opt/nvm
nvm() {
	unset -f nvm
	. "$NVM_DIR/nvm.sh"
	nvm "$@"
}

# ▸ marks a shell inside the cell, which is worth saying in the editor's
# integrated terminal where nothing else does.
PS1='\[\e[32m\]▸\[\e[0m\] \[\e[1;34m\]\w\[\e[0m\] \[\e[32m\]❯\[\e[0m\] '
