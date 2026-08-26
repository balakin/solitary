# Seeded into the cell's home the first time the container started. It is yours
# from then on: edit it, and a rebuilt image will not touch it.

case $- in
*i*) ;;
*) return ;;
esac

export EDITOR=nvim
export VISUAL=nvim
export PATH="$HOME/.local/bin:$PATH"

# Debian's 500-line default is dropped whenever two shells race, and a cell is
# a place where several are open at once.
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

# Dirty state only: untracked scanning is the expensive walk on a large
# repository, and it is the one subprocess in the prompt path.
GIT_PS1_SHOWDIRTYSTATE=1
if [ -r /usr/lib/git-core/git-sh-prompt ]; then
	. /usr/lib/git-core/git-sh-prompt
else
	__git_ps1() { :; }
fi

# ▸ worktree/path* branch ❯   (▸ = inside the cell, * = dirty, ❯ red on error)
#
# The worktree and path come from expanding $PWD rather than asking git, so the
# prompt costs one subprocess instead of two.
__cell_prompt() {
	local rc=$? rest wt rel loc gitinfo branch flags b caret

	case $PWD in
	*__worktrees/*)
		rest=${PWD#*__worktrees/}
		wt=${rest%%/*}
		rel=${rest#"$wt"}
		rel=${rel#/}
		loc=$wt${rel:+/$rel}
		;;
	*)
		wt=
		loc=${PWD/#"$HOME"/\~}
		;;
	esac

	gitinfo=$(__git_ps1 '%s' 2>/dev/null)
	branch=${gitinfo%% *}
	flags=${gitinfo#"$branch"}
	case $flags in
	*[*+%\$]*) flags='*' ;;
	*) flags= ;;
	esac
	if [ "$branch" = "$wt" ]; then b=; else b=${branch:+ $branch}; fi

	# \001/\002 rather than \[ \]: PS1 is assigned from a function, where the
	# \[ \] form is not parsed and would print literally.
	local g=$'\001\e[32m\002' b1=$'\001\e[1;34m\002' y=$'\001\e[33m\002' \
		d=$'\001\e[2;35m\002' r=$'\001\e[31m\002' n=$'\001\e[0m\002'
	if [ "$rc" -eq 0 ]; then caret=$g; else caret=$r; fi

	PS1="${g}▸${n} ${b1}${loc}${n}${y}${flags}${n}${d}${b}${n} ${caret}❯${n} "
}
PROMPT_COMMAND=__cell_prompt
