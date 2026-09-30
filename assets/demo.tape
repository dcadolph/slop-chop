# Record the README demo GIF. Run from the repository root: vhs assets/demo.tape

Output assets/demo.gif

Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 420
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms

Hide
Type "export PATH=$PWD:$PATH PS1='$ '"
Enter
Type "cat > /tmp/inigo.txt <<'EOF'"
Enter
Type "First and foremost, hello. It is important to note that my name is Inigo Montoya."
Enter
Type "Furthermore, you killed my father. In conclusion, prepare to die."
Enter
Type "EOF"
Enter
Type "clear"
Enter
Show

Type "cat /tmp/inigo.txt"
Enter
Sleep 2.5s

Type "slop-chop score /tmp/inigo.txt"
Enter
Sleep 1.5s

Type "slop-chop fix /tmp/inigo.txt"
Enter
Sleep 3.5s

Type "slop-chop fix /tmp/inigo.txt | slop-chop score"
Enter
Sleep 4s
