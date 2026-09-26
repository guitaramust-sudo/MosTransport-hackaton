"""Rig the supplied conductor and export a 30 fps, in-place Idle/Walk GLB.

Run from Blender 5.x in a fresh process. The original GLB is read-only.
"""

import json
import math
import os
from collections import defaultdict
from pathlib import Path

import bpy
from mathutils import Matrix, Quaternion, Vector


HERE = Path(__file__).resolve().parent
SOURCE = Path('/Users/sergejlebedev/Downloads/901887286b53f812d4727b9b2c11433e.glb')
BLEND = HERE / 'conductor_animated.blend'
GLB = HERE / 'conductor_animated.glb'
PREVIEWS = HERE / 'previews'
PREVIEWS.mkdir(parents=True, exist_ok=True)
FPS = 30
WALK_FIRST, WALK_LAST = 1, 31
IDLE_FIRST, IDLE_LAST = 1, 61


def smoothstep(a, b, x):
    t = max(0.0, min(1.0, (x-a)/(b-a)))
    return t*t*(3.0-2.0*t)


def interp(keys, phase):
    for (a, va), (b, vb) in zip(keys, keys[1:]):
        if a <= phase <= b:
            t = smoothstep(a, b, phase)
            return va+(vb-va)*t
    return keys[-1][1]


def foot_trajectory(phase):
    """Compact step; initial contact and early stance are explicitly locked."""
    p = phase % 1.0
    y = interp([(0.0,-0.075), (0.22,-0.075), (0.58,0.075),
                (0.63,0.075), (1.0,-0.075)], p)
    lift = interp([(0.0,0.005), (0.15,0.0), (0.52,0.0),
                   (0.62,0.020), (0.78,0.037), (1.0,0.005)], p)
    pitch = interp([(0.0,-0.023), (0.15,-0.041), (0.50,-0.042),
                    (0.62,-0.060), (0.78,-0.032), (1.0,-0.023)], p)
    return y, lift, pitch


def clear_import():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.ops.import_scene.gltf(filepath=str(SOURCE))
    meshes = [o for o in bpy.context.scene.objects if o.type == 'MESH']
    if len(meshes) != 1 or any(o.type == 'ARMATURE' for o in bpy.context.scene.objects):
        raise RuntimeError('Expected the supplied single unrigged conductor mesh')
    mesh = meshes[0]
    # Bake the source axis conversion into coordinates. The character now has
    # +Z up, -Y forward, origin on the floor, and identity object transforms.
    bpy.ops.object.select_all(action='DESELECT')
    mesh.select_set(True)
    bpy.context.view_layer.objects.active = mesh
    bpy.ops.object.transform_apply(location=False, rotation=True, scale=True)
    mesh.name = 'CharacterMesh'
    mesh.data.name = 'ConductorOriginalGeometry'
    assert max(abs(v) for v in mesh.rotation_euler) < 1e-6
    assert all(abs(v-1.0) < 1e-6 for v in mesh.scale)
    return mesh


def create_hierarchy(mesh):
    scene = bpy.context.scene
    character = bpy.data.collections.new('CHARACTER')
    scene.collection.children.link(character)
    root = bpy.data.objects.new('CharacterRoot', None)
    root.empty_display_type = 'PLAIN_AXES'
    root.empty_display_size = 0.15
    character.objects.link(root)
    arm_data = bpy.data.armatures.new('ConductorSkeleton')
    rig = bpy.data.objects.new('Armature', arm_data)
    character.objects.link(rig)
    rig.parent = root
    rig.show_in_front = True
    arm_data.display_type = 'STICK'
    for col in tuple(mesh.users_collection):
        col.objects.unlink(mesh)
    character.objects.link(mesh)
    mesh.parent = rig
    mesh.matrix_parent_inverse = Matrix.Identity(4)
    mesh.matrix_basis = Matrix.Identity(4)
    return root, rig


def make_skeleton(rig):
    bpy.ops.object.select_all(action='DESELECT')
    rig.select_set(True)
    bpy.context.view_layer.objects.active = rig
    bpy.ops.object.mode_set(mode='EDIT')
    bones = rig.data.edit_bones

    def add(name, head, tail, parent=None, deform=True):
        b = bones.new(name)
        b.head, b.tail = head, tail
        b.use_deform = deform
        if parent:
            b.parent = bones[parent]
            b.use_connect = False
        b.align_roll(Vector((0,1,0)))

    add('Root', (0,0,0), (0,0,0.08), deform=False)
    add('Pelvis', (0,0,0.60), (0,0,0.68), 'Root')
    add('Spine', (0,0,0.68), (0,0,0.82), 'Pelvis')
    add('Chest', (0,0,0.82), (0,0,0.94), 'Spine')
    add('Neck', (0,0,0.94), (0,0,1.01), 'Chest')
    add('Head', (0,0,1.01), (0,0,1.17), 'Neck')
    for side, sign in (('L',-1),('R',1)):
        x = sign*0.100
        add(f'UpperArm.{side}', (x,0,0.91), (sign*0.132,0,0.755), 'Chest')
        add(f'Forearm.{side}', (sign*0.132,0,0.755),
            (sign*0.155,0,0.605), f'UpperArm.{side}')
        add(f'Hand.{side}', (sign*0.155,0,0.605),
            (sign*0.158,0,0.525), f'Forearm.{side}')
        add(f'Thigh.{side}', (sign*0.055,0,0.600),
            (sign*0.055,0,0.330), 'Pelvis')
        add(f'Shin.{side}', (sign*0.055,0,0.330),
            (sign*0.055,0,0.060), f'Thigh.{side}')
        add(f'Foot.{side}', (sign*0.055,0,0.060),
            (sign*0.055,-0.070,0.027), f'Shin.{side}')
        add(f'Toe.{side}', (sign*0.055,-0.070,0.027),
            (sign*0.055,-0.105,0.018), f'Foot.{side}')
    bpy.ops.object.mode_set(mode='OBJECT')
    for pb in rig.pose.bones:
        pb.rotation_mode = 'QUATERNION'


def skin(mesh, rig):
    """Region weights, with disconnected garment patches kept together."""
    groups = {b.name:mesh.vertex_groups.new(name=b.name)
              for b in rig.data.bones if b.use_deform}
    verts=mesh.data.vertices
    parents=list(range(len(verts)))
    def find(i):
        while parents[i]!=i:
            parents[i]=parents[parents[i]]
            i=parents[i]
        return i
    for poly in mesh.data.polygons:
        v=poly.vertices
        for i in v[1:]:
            a,b=find(v[0]),find(i)
            if a!=b:parents[b]=a
    components=defaultdict(list)
    for v in verts:components[find(v.index)].append(v.index)
    arm_side={}
    leg_side={}
    for indices in components.values():
        coords=[verts[i].co for i in indices]
        mx=sum(p.x for p in coords)/len(coords)
        min_z=min(p.z for p in coords)
        max_z=max(p.z for p in coords)
        for side,sign in (('L',-1),('R',1)):
            outer=max(sign*p.x for p in coords)
            if 0.36 < min_z and max_z < 0.975 and sign*mx > 0.088 and outer > 0.120:
                for i in indices:arm_side[i]=side
                break
        if max_z < 0.69 and min_z < 0.55 and abs(mx) > 0.010:
            side='L' if mx < 0 else 'R'
            for i in indices:leg_side[i]=side

    for v in verts:
        x,y,z=v.co
        arm=arm_side.get(v.index)
        weights={}
        if arm:
            hand=1-smoothstep(0.53,0.62,z)
            fore=(1-hand)*(1-smoothstep(0.72,0.78,z))
            upper=max(0.0,1-hand-fore)
            weights={f'Hand.{arm}':hand,f'Forearm.{arm}':fore,
                     f'UpperArm.{arm}':upper}
        else:
            leg=1-smoothstep(0.58,0.665,z)
            body=1-leg
            head=smoothstep(1.005,1.06,z)
            neck=(1-head)*smoothstep(0.955,1.01,z)
            chest=(1-head-neck)*smoothstep(0.81,0.88,z)
            spine=(1-head-neck-chest)*smoothstep(0.68,0.75,z)
            pelvis=max(0.0,1-head-neck-chest-spine)
            for name,w in [('Head',head),('Neck',neck),('Chest',chest),
                           ('Spine',spine),('Pelvis',pelvis)]:
                weights[name]=body*w
            side=leg_side.get(v.index)
            if side:
                shares={side:1.0}
            else:
                left=1-smoothstep(-0.008,0.008,x)
                shares={'L':left,'R':1-left}
            foot=1-smoothstep(0.090,0.140,z)
            toe=foot*smoothstep(0.045,0.095,-y)
            shin=(1-foot)*(1-smoothstep(0.30,0.37,z))
            thigh=max(0.0,1-foot-shin)
            for s,amount in shares.items():
                weights[f'Toe.{s}']=weights.get(f'Toe.{s}',0)+leg*amount*toe
                weights[f'Foot.{s}']=weights.get(f'Foot.{s}',0)+leg*amount*(foot-toe)
                weights[f'Shin.{s}']=weights.get(f'Shin.{s}',0)+leg*amount*shin
                weights[f'Thigh.{s}']=weights.get(f'Thigh.{s}',0)+leg*amount*thigh
        total=sum(weights.values())
        if total < 0.999:
            raise RuntimeError(f'Unweighted vertex {v.index}: {total}')
        for name,w in weights.items():
            if w > 0.0001:groups[name].add([v.index],w/total,'REPLACE')
    modifier=mesh.modifiers.new('ConductorSkin','ARMATURE')
    modifier.object=rig
    modifier.use_deform_preserve_volume=True
    print('SKIN_STATS',json.dumps({'components':len(components),
          'arm_vertices':{s:sum(a==s for a in arm_side.values()) for s in ('L','R')},
          'vertices':len(verts)}))


def bone_matrix(rig,name,head,tail,extra=None):
    bone=rig.data.bones[name]
    head,tail=Vector(head),Vector(tail)
    direction=(tail-head).normalized()
    rest=(bone.tail_local-bone.head_local).normalized()
    q=rest.rotation_difference(direction)
    if extra is not None:
        q=extra@q
    rot=q@bone.matrix_local.to_quaternion()
    pb=rig.pose.bones[name]
    pb.matrix=Matrix.Translation(head)@rot.to_matrix().to_4x4()
    # Child matrices depend on their parent's *evaluated* pose. Updating here
    # avoids a stale-parent first frame and guarantees frame 31 = frame 1.
    bpy.context.view_layer.update()


def rest_point(rig,name,which='head'):
    bone=rig.data.bones[name]
    return Vector(bone.head_local if which=='head' else bone.tail_local)


def leg_knee(hip,ankle,l1,l2):
    h,a=Vector(hip),Vector(ankle)
    dy,dz=a.y-h.y,a.z-h.z
    d=math.hypot(dy,dz)
    d=min(d,l1+l2-0.0001)
    along=(l1*l1-l2*l2+d*d)/(2*d)
    offset=math.sqrt(max(0.0,l1*l1-along*along))
    py,pz=-dz/d,dy/d
    return Vector((h.x,h.y+along*dy/d-offset*py,
                   h.z+along*dz/d-offset*pz))


def pose_frame(rig,phase,idle=False):
    for pb in rig.pose.bones:
        pb.location=(0,0,0)
        pb.rotation_quaternion=(1,0,0,0)
        pb.scale=(1,1,1)
    bpy.context.view_layer.update()
    p=phase%1.0
    if idle:
        breath=math.sin(2*math.pi*p)
        pelvis_shift=Vector((0,0,0.0))
        yaw=0.0
    else:
        pelvis_shift=Vector((0.0035*math.sin(2*math.pi*p),0,
                             -0.008*math.cos(4*math.pi*p)))
        yaw=math.radians(1.4)*math.sin(2*math.pi*p)
    base=rest_point(rig,'Pelvis')
    for name in ('Pelvis','Spine','Chest','Neck','Head'):
        head=rest_point(rig,name)+pelvis_shift
        tail=rest_point(rig,name,'tail')+pelvis_shift
        local_yaw=yaw if name=='Pelvis' else (-0.55*yaw if name=='Chest' else 0)
        if idle and name=='Chest':
            local_yaw=math.radians(0.35)*math.sin(2*math.pi*p)
        extra=Quaternion((0,0,1),local_yaw)
        bone_matrix(rig,name,head,tail,extra)

    for side,sign in (('L',-1),('R',1)):
        shoulder=rest_point(rig,f'UpperArm.{side}')+pelvis_shift
        upper_tail=rest_point(rig,f'UpperArm.{side}','tail')+pelvis_shift
        fore_tail=rest_point(rig,f'Forearm.{side}','tail')+pelvis_shift
        hand_tail=rest_point(rig,f'Hand.{side}','tail')+pelvis_shift
        if idle:
            arm_angle=math.radians(1.0*math.sin(2*math.pi*p))
        else:
            # Left leg forward at phase zero -> right arm forward (-Y).
            arm_angle=math.radians(sign*10.5*math.cos(2*math.pi*p))
        arm_rot=Quaternion((1,0,0),arm_angle)
        upper_dir=arm_rot@(upper_tail-shoulder)
        elbow=shoulder+upper_dir
        bone_matrix(rig,f'UpperArm.{side}',shoulder,elbow)
        fore_rot=Quaternion((1,0,0),arm_angle-math.radians(9.0))
        wrist=elbow+fore_rot@(fore_tail-upper_tail)
        bone_matrix(rig,f'Forearm.{side}',elbow,wrist)
        palm=wrist+fore_rot@(hand_tail-fore_tail)
        bone_matrix(rig,f'Hand.{side}',wrist,palm)

        hip=rest_point(rig,f'Thigh.{side}')+pelvis_shift
        if idle:
            foot_y,lift,pitch=0.0,0.0,-0.040
        else:
            leg_phase=p if side=='L' else (p+0.5)%1.0
            foot_y,lift,pitch=foot_trajectory(leg_phase)
        ankle=Vector((sign*0.055,foot_y,0.068+lift))
        thigh=rig.data.bones[f'Thigh.{side}']
        shin=rig.data.bones[f'Shin.{side}']
        l1=(thigh.tail_local-thigh.head_local).length
        l2=(shin.tail_local-shin.head_local).length
        knee=leg_knee(hip,ankle,l1,l2)
        bone_matrix(rig,f'Thigh.{side}',hip,knee)
        bone_matrix(rig,f'Shin.{side}',knee,ankle)
        toe_base=ankle+Vector((0,-0.070,pitch))
        bone_matrix(rig,f'Foot.{side}',ankle,toe_base)
        bone_matrix(rig,f'Toe.{side}',toe_base,
                    toe_base+Vector((0,-0.035,-0.010)))


def add_action(rig,name,frames,idle=False):
    rig.animation_data_create()
    rig.animation_data.action=None
    action=bpy.data.actions.new(name)
    rig.animation_data.action=action
    action.use_fake_user=True
    scene=bpy.context.scene
    for frame in range(1,frames+1):
        scene.frame_set(frame)
        pose_frame(rig,(frame-1)/(frames-1),idle)
        for pb in rig.pose.bones:
            if pb.name=='Root':continue
            pb.keyframe_insert(data_path='location',frame=frame,group=pb.name)
            pb.keyframe_insert(data_path='rotation_quaternion',frame=frame,group=pb.name)
    return action


def setup_preview(mesh):
    scene=bpy.context.scene
    preview=bpy.data.collections.new('PREVIEW_NOT_EXPORTED')
    scene.collection.children.link(preview)
    world=bpy.data.worlds.new('Preview soft studio')
    world.use_nodes=True
    background=next(n for n in world.node_tree.nodes if n.type=='BACKGROUND')
    background.inputs['Color'].default_value=(0.38,0.43,0.50,1)
    background.inputs['Strength'].default_value=0.8
    scene.world=world
    material=bpy.data.materials.new('Preview ground only')
    material.diffuse_color=(0.35,0.38,0.42,1)
    bpy.ops.mesh.primitive_plane_add(size=200)
    ground=bpy.context.object
    ground.name='Preview ground only'
    for col in tuple(ground.users_collection):col.objects.unlink(ground)
    preview.objects.link(ground)
    ground.location.z=-0.007
    ground.data.materials.append(material)
    for name,location,energy,size in (
        ('Preview key',(2.5,-3.2,4.4),430,4.0),
        ('Preview fill',(-2.8,-1.0,3.4),250,3.7),
        ('Preview rim',(0,2.5,3.8),360,3.0),
    ):
        data=bpy.data.lights.new(name,'AREA')
        data.energy=energy
        data.shape='DISK'
        data.size=size
        ob=bpy.data.objects.new(name,data)
        preview.objects.link(ob)
        ob.location=location
        ob.rotation_euler=(Vector((0,0,0.6))-ob.location).to_track_quat('-Z','Y').to_euler()
    cameras={}
    for name,location in (
        ('side',(2.4,0,0.72)),
        ('front',(0,-2.4,0.72)),
        ('gameplay',(1.9,-2.1,2.5)),
    ):
        data=bpy.data.cameras.new(f'CAM_Walk_{name}')
        ob=bpy.data.objects.new(f'CAM_Walk_{name}',data)
        preview.objects.link(ob)
        ob.location=location
        target=Vector((0,0,0.60))
        ob.rotation_euler=(target-ob.location).to_track_quat('-Z','Y').to_euler()
        data.type='ORTHO'
        data.ortho_scale=1.62 if name!='gameplay' else 1.70
        cameras[name]=ob
    scene.camera=cameras['gameplay']
    scene.render.engine='BLENDER_EEVEE'
    scene.render.resolution_x=540
    scene.render.resolution_y=640
    scene.render.resolution_percentage=100
    scene.render.image_settings.file_format='PNG'
    scene.view_settings.view_transform='AgX'
    return cameras


def export_character(root,rig,mesh,walk):
    bpy.ops.object.select_all(action='DESELECT')
    for ob in (root,rig,mesh):ob.select_set(True)
    bpy.context.view_layer.objects.active=rig
    rig.animation_data.action=walk
    bpy.ops.export_scene.gltf(filepath=str(GLB),export_format='GLB',
                              use_selection=True,export_animations=True,
                              export_animation_mode='BROADCAST',
                              export_skins=True,export_yup=True,
                              export_force_sampling=True,
                              export_anim_slide_to_zero=True,
                              export_frame_step=1,
                              export_def_bones=False)


def render_checks(scene,rig,walk,cameras):
    rig.animation_data.action=walk
    for name in ('front','side','gameplay'):
        scene.camera=cameras[name]
        for frame in (1,5,8,12,16,20,23,27):
            scene.frame_set(frame)
            scene.render.filepath=str(PREVIEWS/f'check_{name}_{frame:02d}.png')
            bpy.ops.render.render(write_still=True)


def render_video_frames(scene,rig,walk,cameras):
    rig.animation_data.action=walk
    scene.frame_start=1
    scene.frame_end=30
    scene.render.fps=30
    scene.render.image_settings.file_format='PNG'
    for name in ('side','front','gameplay'):
        scene.camera=cameras[name]
        folder=PREVIEWS/f'frames_{name}'
        folder.mkdir(exist_ok=True)
        for frame in range(1,31):
            scene.frame_set(frame)
            scene.render.filepath=str(folder/f'{frame:04d}.png')
            bpy.ops.render.render(write_still=True)


def main():
    if not SOURCE.is_file():raise FileNotFoundError(SOURCE)
    mesh=clear_import()
    root,rig=create_hierarchy(mesh)
    make_skeleton(rig)
    skin(mesh,rig)
    idle=add_action(rig,'Idle',IDLE_LAST,idle=True)
    walk=add_action(rig,'Walk',WALK_LAST,idle=False)
    rig.animation_data.action=walk
    scene=bpy.context.scene
    scene.render.fps=FPS
    scene.frame_start=WALK_FIRST
    scene.frame_end=WALK_LAST
    scene.frame_set(1)
    rig['forward_blender']='-Y'
    rig['forward_gltf']='+Z'
    rig['walk']='In-place, 30 frames played in a 1–30 loop; frame 31 duplicates 1'
    cameras=setup_preview(mesh)
    scene.camera=cameras['gameplay']
    bpy.ops.file.pack_all()
    bpy.ops.wm.save_as_mainfile(filepath=str(BLEND))
    export_character(root,rig,mesh,walk)
    if os.environ.get('CONDUCTOR_CHECK_RENDERS')=='1':
        render_checks(scene,rig,walk,cameras)
    if os.environ.get('CONDUCTOR_RENDER_VIDEOS')=='1':
        render_video_frames(scene,rig,walk,cameras)
    print('CONDUCTOR_RESULT',json.dumps({'blend':str(BLEND),'glb':str(GLB),
          'bones':len(rig.data.bones),'actions':[a.name for a in bpy.data.actions],
          'vertices':len(mesh.data.vertices),'polygons':len(mesh.data.polygons)}))


if __name__=='__main__':main()
