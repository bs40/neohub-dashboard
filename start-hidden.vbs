' Starts the neoHub dashboard server with no console window. Used by the "neoHub Dashboard" scheduled task.
Set sh = CreateObject("WScript.Shell")
sh.CurrentDirectory = "S:\Projects\neohub-dashboard"
sh.Run """C:\Program Files\nodejs\node.exe"" ""S:\Projects\neohub-dashboard\server.js""", 0, False
