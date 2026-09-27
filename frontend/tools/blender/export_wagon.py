"""Export the wagon scene used by Expo from the supplied Blender project."""

import bpy
from pathlib import Path


models = Path(__file__).resolve().parents[2] / "assets" / "models"
source = models / "TOOOPblend.blend"
target = models / "TOOOPblend.glb"

bpy.ops.wm.open_mainfile(filepath=str(source))
bpy.ops.export_scene.gltf(
    filepath=str(target),
    export_format="GLB",
    use_selection=False,
    export_cameras=False,
    export_lights=False,
    export_yup=True,
)
print(f"WAGON_EXPORT {target} bytes={target.stat().st_size}")
