"""Agente que ignora el cierre de stdin para forzar el camino de kill."""
import signal
import sys
import time

signal.signal(signal.SIGTERM, signal.SIG_IGN)
sys.stdin.close()
while True:
    time.sleep(1)
