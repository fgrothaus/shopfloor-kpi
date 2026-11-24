"""
OPC-UA-Mockserver für Maschinendaten (Lernprojekt MDE / Telemetrie)

Simuliert eine Stanzpresse mit:
  - Zustandsautomat: Aus, Leerlauf, Produktion, Störung, Rüsten
  - Zählern:         Hübe, Gutteile, Ausschuss
  - Prozesswerten:   Presskraft, Öltemperatur, Wirkleistung, Energiezähler
  - Auftragsdaten:   Auftragsnummer, Sollmenge, Soll-Taktzeit

Jede Maschine bekommt einen EIGENEN OPC-UA-Server auf eigenem Port,
so wie in der Realität jede SPS ihren eigenen Server hat.
Weitere Maschinen trägst du einfach in MACHINES ein.

Start:
    pip install -r requirements.txt
    python mock_server.py

Endpoint:  opc.tcp://localhost:4840
NodeIds:   ns=2;s=Presse_01.State, ns=2;s=Presse_01.StrokeCounter, ...
"""

import asyncio
import logging
import random
from dataclasses import dataclass
from datetime import datetime, timezone
from enum import IntEnum

from asyncua import Server, ua

# ---------------------------------------------------------------------------
# Konfiguration 
# ---------------------------------------------------------------------------

NAMESPACE_URI = "urn:mock:machines"
TICK_SECONDS = 0.2  # Simulationstakt (5 Hz)


@dataclass
class MachineConfig:
    name: str
    port: int
    target_cycle_time: float = 4.2   # Soll-Taktzeit in Sekunden pro Hub
    order_quantity: int = 150        # Sollmenge pro Auftrag
    scrap_rate: float = 0.02         # 2 % Ausschuss
    mtbf_seconds: float = 300.0      # mittlere Zeit zwischen Störungen


MACHINES = [
    MachineConfig(name="Presse_01", port=4840),
    MachineConfig(name="Presse_02", port=4841, target_cycle_time=3.8),
]

ALARM_CODES = {
    101: "Materialzufuhr gestört",
    205: "Lichtschranke unterbrochen",
    312: "Hydraulikdruck zu niedrig",
    418: "Werkzeugbruch erkannt",
}

ALARM_SEVERITY = {
    101: 500,
    205: 400,
    312: 700,
    418: 900,
}

class State(IntEnum):
    AUS = 0
    LEERLAUF = 1
    PRODUKTION = 2
    STOERUNG = 3
    RUESTEN = 4


# ---------------------------------------------------------------------------
# Simulation (reine Logik, weiß nichts von OPC UA)
# ---------------------------------------------------------------------------


class PressSimulation:
    def __init__(self, cfg: MachineConfig):
        self.cfg = cfg

        # Zustand
        self.state = State.LEERLAUF
        self.state_remaining = random.uniform(5, 15)  # Sekunden bis zum nächsten Wechsel
        self.alarm_code = 0

        # Zähler (laufen über die gesamte Lebensdauer, wie in einer echten SPS)
        self.stroke_counter = 0
        self.good_parts = 0
        self.scrap_parts = 0

        # Prozesswerte
        self.press_force = 0.0      # kN
        self.oil_temperature = 28.0  # °C
        self.active_power = 0.0     # kW
        self.energy_total = 0.0     # kWh

        # Auftrag
        self.order_counter = 1
        self.order_number = self._new_order_number()
        self.order_good = 0

        # intern
        self._cycle_progress = 0.0
        self._next_cycle = self._jittered_cycle()
        self._tool_wear = 0.0  # steigt pro Hub, wird beim Rüsten zurückgesetzt

    # --- Hilfsfunktionen ---------------------------------------------------

    def _new_order_number(self) -> str:
        return f"FA-{datetime.now():%Y%m%d}-{self.order_counter:04d}"

    def _jittered_cycle(self) -> float:
        # Echte Takte schwanken leicht und sind im Schnitt etwas langsamer als Soll
        return self.cfg.target_cycle_time * random.uniform(1.0, 1.12)

    def _set_state(self, new_state: State, duration: float, alarm_code: int = 0):
        old = self.state
        self.state = new_state
        self.state_remaining = duration
        self.alarm_code = alarm_code
        info = f" (Alarm {alarm_code}: {ALARM_CODES[alarm_code]})" if alarm_code else ""
        logging.info("%s: %s -> %s%s", self.cfg.name, old.name, new_state.name, info)

    # --- ein Simulationsschritt --------------------------------------------

    def step(self, dt: float):
        self.state_remaining -= dt
        self._update_state(dt)
        self._update_process_values(dt)

    def _update_state(self, dt: float):
        if self.state == State.PRODUKTION:
            # Zufällige Störung (Poisson-Prozess über MTBF)
            if random.random() < dt / self.cfg.mtbf_seconds:
                code = random.choice(list(ALARM_CODES))
                self._set_state(State.STOERUNG, random.uniform(20, 120), code)
                return

            # Kurzer Leerlauf, z. B. Materialwechsel durch den Werker
            if random.random() < dt / 600:
                self._set_state(State.LEERLAUF, random.uniform(10, 40))
                return

            # Takt abarbeiten
            self._cycle_progress += dt
            if self._cycle_progress >= self._next_cycle:
                self._cycle_progress = 0.0
                self._next_cycle = self._jittered_cycle()
                self._do_stroke()

            # Auftrag fertig → Rüsten auf den nächsten Auftrag
            if self.order_good >= self.cfg.order_quantity:
                self._set_state(State.RUESTEN, random.uniform(60, 180))

        elif self.state_remaining <= 0:
            if self.state == State.RUESTEN:
                # Neuer Auftrag, neues Werkzeug
                self.order_counter += 1
                self.order_number = self._new_order_number()
                self.order_good = 0
                self._tool_wear = 0.0
                self._set_state(State.LEERLAUF, random.uniform(5, 15))
            elif self.state == State.STOERUNG:
                # Werker hat quittiert, kurzer Leerlauf vor Wiederanlauf
                self._set_state(State.LEERLAUF, random.uniform(5, 20))
            elif self.state == State.LEERLAUF:
                self._set_state(State.PRODUKTION, 0)

    def _do_stroke(self):
        self.stroke_counter += 1
        self._tool_wear += 0.05  # Presskraft steigt langsam mit Verschleiß

        # Ausschuss wird mit Verschleiß leicht wahrscheinlicher
        scrap_probability = self.cfg.scrap_rate + self._tool_wear / 2000
        if random.random() < scrap_probability:
            self.scrap_parts += 1
        else:
            self.good_parts += 1
            self.order_good += 1

        self.press_force = round(800 + self._tool_wear + random.gauss(0, 6), 1)

    def _update_process_values(self, dt: float):
        # Öltemperatur nähert sich langsam einem Zielwert (Trägheit, tau = 120 s)
        target_temp = {State.PRODUKTION: 55.0, State.AUS: 22.0}.get(self.state, 35.0)
        self.oil_temperature += (target_temp - self.oil_temperature) * dt / 120
        self.oil_temperature += random.gauss(0, 0.02)

        # Wirkleistung je nach Zustand
        base_power = {
            State.AUS: 0.0,
            State.LEERLAUF: 9.0,
            State.PRODUKTION: 42.0,
            State.STOERUNG: 8.0,
            State.RUESTEN: 5.0,
        }[self.state]
        noise = random.gauss(0, base_power * 0.03) if base_power else 0.0
        self.active_power = max(0.0, base_power + noise)

        # Energiezähler integriert die Leistung (kW * h = kWh)
        self.energy_total += self.active_power * dt / 3600

    def snapshot(self) -> dict:
        """Aktuelle Werte, gerundet wie eine echte SPS sie liefern würde."""
        return {
            "State": int(self.state),
            "AlarmCode": self.alarm_code,
            "StrokeCounter": self.stroke_counter,
            "GoodParts": self.good_parts,
            "ScrapParts": self.scrap_parts,
            "PressForce": self.press_force,
            "OilTemperature": round(self.oil_temperature, 1),
            "ActivePower": round(self.active_power, 1),
            "EnergyTotal": round(self.energy_total, 3),
            "OrderNumber": self.order_number,
            "TargetQuantity": self.cfg.order_quantity,
            "TargetCycleTime": self.cfg.target_cycle_time,
        }


# ---------------------------------------------------------------------------
# OPC-UA-Schicht
# ---------------------------------------------------------------------------

# Node-Name -> OPC-UA-Datentyp (wie er auch in einer SPS-Variablenliste stünde)
NODE_TYPES = {
    "State": ua.VariantType.Int32,
    "AlarmCode": ua.VariantType.Int32,
    "StrokeCounter": ua.VariantType.UInt32,
    "GoodParts": ua.VariantType.UInt32,
    "ScrapParts": ua.VariantType.UInt32,
    "PressForce": ua.VariantType.Double,      # kN
    "OilTemperature": ua.VariantType.Double,  # °C
    "ActivePower": ua.VariantType.Double,     # kW
    "EnergyTotal": ua.VariantType.Double,     # kWh
    "OrderNumber": ua.VariantType.String,
    "TargetQuantity": ua.VariantType.UInt32,
    "TargetCycleTime": ua.VariantType.Double,  # s
}

async def send_alarm_event(evgen, cfg: MachineConfig, code: int, active: bool):
    evgen.event.AlarmCode = ua.Variant(code, ua.VariantType.Int32)
    evgen.event.Active = ua.Variant(active, ua.VariantType.Boolean)
    evgen.event.Severity = ALARM_SEVERITY[code] if active else 100
    evgen.event.SourceName = cfg.name
    status = "gekommen" if active else "gegangen"
    await evgen.trigger(message=f"{code}: {ALARM_CODES[code]} ({status})")
    logging.info("%s: Event gesendet – Alarm %d %s", cfg.name, code, status)

async def run_machine(cfg: MachineConfig):
    sim = PressSimulation(cfg)

    server = Server()
    await server.init()
    server.set_endpoint(f"opc.tcp://0.0.0.0:{cfg.port}")
    server.set_server_name(f"Mock {cfg.name}")
    server.set_security_policy([ua.SecurityPolicyType.NoSecurity])

    idx = await server.register_namespace(NAMESPACE_URI)
    machine_obj = await server.nodes.objects.add_object(idx, cfg.name)


    # NEU: eigener Event-Typ, abgeleitet von BaseEventType, mit zwei Zusatzfeldern
    alarm_event_type = await server.create_custom_event_type(
        idx,
        "MachineAlarmEventType",
        ua.ObjectIds.BaseEventType,
        [
            ("AlarmCode", ua.VariantType.Int32),
            ("Active", ua.VariantType.Boolean),
        ],
    )
    # NEU: Generator mit der Presse als Quelle (setzt dort auch den EventNotifier)
    alarm_evgen = await server.get_event_generator(alarm_event_type, machine_obj)


    # Variablen mit sprechenden String-NodeIds anlegen: ns=2;s=Presse_01.State
    nodes = {}
    for name, vtype in NODE_TYPES.items():
        initial = sim.snapshot()[name]
        node = await machine_obj.add_variable(
            ua.NodeId(f"{cfg.name}.{name}", idx),
            ua.QualifiedName(name, idx),
            ua.Variant(initial, vtype),
        )
        nodes[name] = node

    async with server:
        logging.info("%s läuft auf opc.tcp://localhost:%d (ns=%d)", cfg.name, cfg.port, idx)
        last_values = {}
        last_alarm_code = 0

        while True:
            sim.step(TICK_SECONDS)
            now = datetime.now(timezone.utc)

            # Nur geänderte Werte schreiben, mit Quell-Zeitstempel wie eine echte SPS
            for name, value in sim.snapshot().items():
                if last_values.get(name) == value:
                    continue
                last_values[name] = value
                dv = ua.DataValue(
                    ua.Variant(value, NODE_TYPES[name]),
                    SourceTimestamp=now,
                    ServerTimestamp=now,
                )
                await nodes[name].write_value(dv)

            if sim.alarm_code != last_alarm_code:
                if last_alarm_code:   # alter Alarm ist weg
                    await send_alarm_event(alarm_evgen, cfg, last_alarm_code, active=False)
                if sim.alarm_code:    # neuer Alarm ist da
                    await send_alarm_event(alarm_evgen, cfg, sim.alarm_code, active=True)
                last_alarm_code = sim.alarm_code

            await asyncio.sleep(TICK_SECONDS)


async def main():
    logging.basicConfig(level=logging.INFO, format="%(asctime)s  %(message)s", datefmt="%H:%M:%S")
    logging.getLogger("asyncua").setLevel(logging.WARNING)
    await asyncio.gather(*(run_machine(cfg) for cfg in MACHINES))


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nMock-Server beendet.")