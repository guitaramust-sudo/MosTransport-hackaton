"""Validate the delivered Blender scene and reimported GLB in Blender 5.x."""

import json
import math
import struct
from pathlib import Path

import bpy

HERE=Path(__file__).resolve().parent
BLEND=HERE/'conductor_animated.blend'
GLB=HERE/'conductor_animated.glb'

def mesh_positions(mesh,frame):
    base=math.floor(frame)
    bpy.context.scene.frame_set(base,subframe=frame-base)
    dg=bpy.context.evaluated_depsgraph_get()
    ob=mesh.evaluated_get(dg)
    data=ob.to_mesh()
    points=[ob.matrix_world@v.co for v in data.vertices]
    ob.to_mesh_clear()
    return points

def gap(a,b):
    return max((u-v).length for u,v in zip(a,b))

bpy.ops.wm.open_mainfile(filepath=str(BLEND))
root=bpy.data.objects['CharacterRoot']
rig=bpy.data.objects['Armature']
mesh=bpy.data.objects['CharacterMesh']
assert mesh.parent==rig and rig.parent==root
assert len(rig.data.bones)==20
assert {a.name for a in bpy.data.actions}=={'Idle','Walk'}
assert len(mesh.data.materials)==1
assert len(mesh.data.vertices)==31752
assert len(mesh.data.polygons)==50000
source_vertices=len(mesh.data.vertices)
assert all(abs(x)<1e-7 for x in root.location)
assert all(abs(x-1)<1e-7 for x in root.scale)
assert all(abs(x-1)<1e-7 for x in rig.scale)

rig.animation_data.action=bpy.data.actions['Walk']
p1=mesh_positions(mesh,1)
p31=mesh_positions(mesh,31)
p16=mesh_positions(mesh,16)
loop_gap=gap(p1,p31)
motion=gap(p1,p16)
assert loop_gap<0.0001,(loop_gap,'loop')
assert motion>0.025,(motion,'frozen')
min_floor=min(min(p.z for p in mesh_positions(mesh,f)) for f in (1,5,8,12,16,20,23,27,31))
assert min_floor>-0.03,min_floor
feet={}
for side in ('L','R'):
    ys=[]
    for f in range(1,8):
        bpy.context.scene.frame_set(f)
        ys.append(rig.pose.bones[f'Foot.{side}'].head.y)
    feet[side]=ys
assert max(feet['L'])-min(feet['L'])<0.001,feet['L']
assert all(abs(v)<1e-7 for v in root.location)

with GLB.open('rb') as file:
    magic,version,total=struct.unpack('<4sII',file.read(12))
    n,typ=struct.unpack('<I4s',file.read(8))
    gltf=json.loads(file.read(n))
assert magic==b'glTF' and version==2
assert len(gltf.get('skins',[]))==1
assert [a['name'] for a in gltf['animations']]==['Idle','Walk']
names=[n.get('name','') for n in gltf['nodes']]
assert 'CharacterRoot' in names and 'CharacterMesh' in names
for action in gltf['animations']:
    assert not any(names[c['target']['node']]=='CharacterRoot' for c in action['channels'])
assert len(gltf.get('materials',[]))==1 and len(gltf.get('images',[]))>=1

bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=str(GLB))
rig2=next(o for o in bpy.context.scene.objects if o.type=='ARMATURE')
mesh2=next(o for o in bpy.context.scene.objects if o.type=='MESH')
root2=bpy.data.objects.get('CharacterRoot')
assert root2 is not None
assert len(rig2.data.bones)==20
assert len(mesh2.data.materials)==1
actions={a.name:a for a in bpy.data.actions}
assert set(actions)=={'Idle','Walk'},set(actions)
walk=actions['Walk']
rig2.animation_data.action=walk
start,end=walk.frame_range[:]
q0=mesh_positions(mesh2,start)
q1=mesh_positions(mesh2,end)
qm=mesh_positions(mesh2,(start+end)/2)
import_gap=gap(q0,q1)
import_motion=gap(q0,qm)
assert import_gap<0.002,(import_gap,'reimport loop')
assert import_motion>0.025,(import_motion,'reimport motion')
assert all(abs(v)<1e-7 for v in root2.location)

result={
    'source_vertices':source_vertices,
    'bones':len(rig2.data.bones),
    'actions':sorted(actions),
    'blend_loop_gap_m':loop_gap,
    'glb_loop_gap_m':import_gap,
    'glb_midcycle_vertex_motion_m':import_motion,
    'walk_action_frames':[start,end],
    'left_foot_early_stance_range_m':max(feet['L'])-min(feet['L']),
    'lowest_sampled_vertex_z_m':min_floor,
    'root_motion':False,
    'materials':len(mesh2.data.materials),
    'texture_images':len(gltf.get('images',[])),
}
(HERE/'verification.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
print('VERIFY_CONDUCTOR',json.dumps(result))
