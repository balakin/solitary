# Seeded into the cell's home the first time the container started. It is yours
# from then on: edit it, and a rebuilt image will not touch it.
#
# tmux starts every pane as a login shell, and a login shell reads this rather
# than ~/.bashrc — without it a pane gets Ubuntu's default prompt and none of
# what ~/.bashrc sets up.
if [ -n "$BASH_VERSION" ] && [ -r "$HOME/.bashrc" ]; then
	. "$HOME/.bashrc"
fi
