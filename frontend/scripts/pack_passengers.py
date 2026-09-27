#!/usr/bin/env python3
"""Pack supplied passenger GLBs into one four-clip GLB per character.

Usage: python3 scripts/pack_passengers.py /path/to/original/glbs
The IdleSeated (1)/(2) exports are the updated seated poses. The other three
exports supply animations only; geometry and embedded textures are stored once.
"""

import argparse
import copy
import hashlib
import json
import struct
import subprocess
import tempfile
from pathlib import Path


CHARACTERS = ('new_01', 'new_02', 'new_03', 'new_04', 'new_05', 'new_06', 'male', 'female')
CLIPS = ('SitDown', 'StandUp', 'Walk')


def read_glb(path: Path):
    content = path.read_bytes()
    magic, version, length = struct.unpack_from('<4sII', content)
    if magic != b'glTF' or version != 2 or length != len(content):
        raise ValueError(f'Invalid GLB: {path}')
    json_length, json_type = struct.unpack_from('<I4s', content, 12)
    if json_type != b'JSON':
        raise ValueError(f'Missing JSON chunk: {path}')
    document = json.loads(content[20:20 + json_length])
    binary_at = 20 + json_length
    binary_length, binary_type = struct.unpack_from('<I4s', content, binary_at)
    if binary_type != b'BIN\0':
        raise ValueError(f'Missing binary chunk: {path}')
    binary = content[binary_at + 8:binary_at + 8 + binary_length]
    return document, binary


def view_bytes(document, binary, index):
    view = document['bufferViews'][index]
    start = view.get('byteOffset', 0)
    return binary[start:start + view['byteLength']]


def accessor_bytes(document, binary, index):
    accessor = document['accessors'][index]
    return view_bytes(document, binary, accessor['bufferView'])


def check_rig(base, base_binary, source, source_binary, filename):
    if [node.get('name') for node in source['nodes']] != [node.get('name') for node in base['nodes']]:
        raise ValueError(f'Bone order differs: {filename}')
    if source['skins'] != base['skins']:
        raise ValueError(f'Skeleton differs: {filename}')
    if len(source['images']) != len(base['images']):
        raise ValueError(f'Texture count differs: {filename}')
    for old, new in zip(base['images'], source['images']):
        if hashlib.sha256(view_bytes(base, base_binary, old['bufferView'])).digest() != hashlib.sha256(view_bytes(source, source_binary, new['bufferView'])).digest():
            raise ValueError(f'Texture differs: {filename}')
    base_primitive = base['meshes'][0]['primitives'][0]
    source_primitive = source['meshes'][0]['primitives'][0]
    for semantic in ('POSITION', 'NORMAL', 'TEXCOORD_0'):
        if accessor_bytes(base, base_binary, base_primitive['attributes'][semantic]) != accessor_bytes(source, source_binary, source_primitive['attributes'][semantic]):
            raise ValueError(f'{semantic} differs: {filename}')
    if accessor_bytes(base, base_binary, base_primitive['indices']) != accessor_bytes(source, source_binary, source_primitive['indices']):
        raise ValueError(f'Triangles differ: {filename}')


def append_animation(base, output_binary, source, source_binary, expected_name):
    animation = copy.deepcopy(source['animations'][0])
    if animation['name'] != expected_name or len(animation['channels']) == 0:
        raise ValueError(f'Expected a nonempty {expected_name} animation')
    view_map = {}
    accessor_map = {}

    def copy_accessor(index):
        if index in accessor_map:
            return accessor_map[index]
        accessor = copy.deepcopy(source['accessors'][index])
        source_view_index = accessor['bufferView']
        if source_view_index not in view_map:
            while len(output_binary) % 4:
                output_binary.append(0)
            view = copy.deepcopy(source['bufferViews'][source_view_index])
            view['byteOffset'] = len(output_binary)
            output_binary.extend(view_bytes(source, source_binary, source_view_index))
            view_map[source_view_index] = len(base['bufferViews'])
            base['bufferViews'].append(view)
        accessor['bufferView'] = view_map[source_view_index]
        accessor_map[index] = len(base['accessors'])
        base['accessors'].append(accessor)
        return accessor_map[index]

    for sampler in animation['samplers']:
        sampler['input'] = copy_accessor(sampler['input'])
        sampler['output'] = copy_accessor(sampler['output'])
    base['animations'].append(animation)


def resize_embedded_textures(document, binary, max_size):
    """Downsample the 4K PNGs for the mobile scene without changing channels."""
    if not max_size:
        return binary
    replacements = {}
    with tempfile.TemporaryDirectory() as temp:
        for image in document['images']:
            if image.get('mimeType') != 'image/png':
                raise ValueError('Only embedded PNG passenger textures are supported')
            index = image['bufferView']
            original = view_bytes(document, binary, index)
            source = Path(temp) / f'{index}.png'
            target = Path(temp) / f'{index}-small.png'
            source.write_bytes(original)
            subprocess.run(['sips', '-s', 'format', 'png', '-Z', str(max_size), str(source), '--out', str(target)],
                           check=True, stdout=subprocess.DEVNULL)
            replacement = target.read_bytes()
            if replacement[:8] != b'\x89PNG\r\n\x1a\n':
                raise ValueError(f'Invalid resized PNG: {target}')
            width, height = struct.unpack_from('>II', replacement, 16)
            if max(width, height) > max_size:
                raise ValueError(f'Texture still exceeds {max_size}px: {target}')
            replacements[index] = replacement

    rebuilt = bytearray()
    for index, view in enumerate(document['bufferViews']):
        while len(rebuilt) % 4:
            rebuilt.append(0)
        data = replacements.get(index)
        if data is None:
            data = view_bytes(document, binary, index)
        # Read all old offsets before changing any of them.
        view['newByteOffset'] = len(rebuilt)
        view['newByteLength'] = len(data)
        rebuilt.extend(data)
    for view in document['bufferViews']:
        view['byteOffset'] = view.pop('newByteOffset')
        view['byteLength'] = view.pop('newByteLength')
    return rebuilt


def write_glb(path, document, binary):
    while len(binary) % 4:
        binary.append(0)
    document['buffers'][0]['byteLength'] = len(binary)
    encoded = json.dumps(document, separators=(',', ':'), ensure_ascii=False).encode('utf-8')
    encoded += b' ' * ((-len(encoded)) % 4)
    length = 12 + 8 + len(encoded) + 8 + len(binary)
    with path.open('wb') as output:
        output.write(struct.pack('<4sII', b'glTF', 2, length))
        output.write(struct.pack('<I4s', len(encoded), b'JSON'))
        output.write(encoded)
        output.write(struct.pack('<I4s', len(binary), b'BIN\0'))
        output.write(binary)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path, help='directory containing the supplied GLBs')
    parser.add_argument('--texture-size', type=int, default=1024,
                        help='maximum embedded texture dimension; 0 preserves originals (default: 1024)')
    args = parser.parse_args()
    destination = Path(__file__).resolve().parents[1] / 'assets' / 'characters'
    destination.mkdir(parents=True, exist_ok=True)
    for character in CHARACTERS:
        seated_suffix = '(2)' if character == 'female' else '(1)'
        seated_path = args.source / f'{character}_IdleSeated {seated_suffix}.glb'
        document, binary = read_glb(seated_path)
        output_binary = bytearray(binary)
        if len(document.get('animations', [])) != 1 or document['animations'][0]['name'] != 'IdleSeated':
            raise ValueError(f'Expected updated IdleSeated clip in {seated_path}')
        for name in CLIPS:
            path = args.source / f'{character}_{name}.glb'
            source, source_binary = read_glb(path)
            check_rig(document, binary, source, source_binary, path)
            append_animation(document, output_binary, source, source_binary, name)
        # Exports use an invalid specular factor of 2. glTF permits 0..1.
        # Clamp it so the textures retain their intended colours under lights.
        for material in document.get('materials', []):
            specular = material.get('extensions', {}).get('KHR_materials_specular', {})
            if 'specularColorFactor' in specular:
                specular['specularColorFactor'] = [min(1, max(0, value)) for value in specular['specularColorFactor']]
        output_binary = resize_embedded_textures(document, output_binary, args.texture_size)
        output = destination / f'passenger_{character}.glb'
        write_glb(output, document, output_binary)
        print(f'{output.name}: {output.stat().st_size / 1_000_000:.1f} MB, {len(document["animations"])} clips')


if __name__ == '__main__':
    main()
