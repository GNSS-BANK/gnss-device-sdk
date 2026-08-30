package plot

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2/canvas"
)

const encodedCoordinateScale = 3.0
const encodedCoordinateOffset = 1.0

// plotShaderSource рисует одну серию. Точки передаются как RGBA-текстура:
// RG содержат 16-битный X, BA — 16-битный Y.
const plotShaderSource = `#version 110

uniform vec2 frame;
uniform vec4 bounds;
uniform sampler2D points;
uniform float pointCount;
uniform float textureWidth;
uniform float drawMode;
uniform float lineWidth;
uniform float pointRadius;
uniform float stemBase;
uniform float colorR;
uniform float colorG;
uniform float colorB;
uniform float colorA;

float decode16(vec2 value) {
    return (value.x * 255.0 * 256.0 + value.y * 255.0) / 65535.0;
}

vec2 pointAt(float index) {
    vec4 texel = texture2D(points, vec2((index + 0.5) / textureWidth, 0.5));
    return vec2(decode16(texel.rg), decode16(texel.ba)) * 3.0 - 1.0;
}

float segmentDistance(vec2 p, vec2 a, vec2 b) {
    vec2 ab = b - a;
    float denominator = max(dot(ab, ab), 0.000001);
    float t = clamp(dot(p - a, ab) / denominator, 0.0, 1.0);
    return length(p - (a + ab * t));
}

float lowerBound(float x) {
    float lo = 0.0;
    float hi = pointCount;
    for (int i = 0; i < 14; i++) {
		if (lo >= hi) {
			break;
		}
        float mid = floor((lo + hi) * 0.5);
        if (mid >= hi) {
            continue;
        }
        if (pointAt(mid).x < x) {
            lo = mid + 1.0;
        } else {
            hi = mid;
        }
    }
    return min(lo, pointCount - 1.0);
}

void main() {
    if (pointCount < 1.0) {
        discard;
    }
    vec2 size = vec2(bounds[2] - bounds[0], bounds[3] - bounds[1]);
    vec2 origin = vec2(bounds[0], frame.y - bounds[3]);
    vec2 uv = (gl_FragCoord.xy - origin) / size;
    vec2 pixel = uv * size;
    float index = lowerBound(uv.x);
    float distancePx = 1000000.0;

    if (drawMode < 0.5) {
        if (pointCount == 1.0) {
            distancePx = length(pixel - pointAt(0.0) * size);
        } else {
            float left = max(index - 1.0, 0.0);
            float right = min(index, pointCount - 1.0);
            distancePx = segmentDistance(pixel, pointAt(left) * size, pointAt(right) * size);
            if (right + 1.0 < pointCount) {
                distancePx = min(distancePx, segmentDistance(pixel, pointAt(right) * size, pointAt(right + 1.0) * size));
            }
        }
    } else if (drawMode < 1.5) {
        for (int offset = -2; offset <= 2; offset++) {
            float candidate = clamp(index + float(offset), 0.0, pointCount - 1.0);
            distancePx = min(distancePx, length(pixel - pointAt(candidate) * size));
        }
    } else {
        for (int offset = -1; offset <= 1; offset++) {
            float candidate = clamp(index + float(offset), 0.0, pointCount - 1.0);
            vec2 value = pointAt(candidate);
            vec2 stemEnd = vec2(value.x, stemBase);
            distancePx = min(distancePx, segmentDistance(pixel, vec2(value.x, stemBase) * size, value * size));
            distancePx = min(distancePx, length(pixel - value * size));
        }
    }

    float radius = drawMode < 0.5 ? max(lineWidth * 0.5, 0.5) : pointRadius;
    if (drawMode > 1.5) {
        radius = max(radius, lineWidth * 0.5);
    }
    float alpha = 1.0 - smoothstep(radius, radius + 1.0, distancePx);
    if (alpha <= 0.0) {
        discard;
    }
    gl_FragColor = vec4(colorR, colorG, colorB, colorA * alpha);
}
`

const plotShaderSourceES = `#version 100

#ifdef GL_ES
# ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
# else
precision mediump float;
# endif
precision mediump int;
#endif

uniform vec2 frame;
uniform vec4 bounds;
uniform sampler2D points;
uniform float pointCount;
uniform float textureWidth;
uniform float drawMode;
uniform float lineWidth;
uniform float pointRadius;
uniform float stemBase;
uniform float colorR;
uniform float colorG;
uniform float colorB;
uniform float colorA;

float decode16(vec2 value) {
    return (value.x * 255.0 * 256.0 + value.y * 255.0) / 65535.0;
}

vec2 pointAt(float index) {
    vec4 texel = texture2D(points, vec2((index + 0.5) / textureWidth, 0.5));
    return vec2(decode16(texel.rg), decode16(texel.ba)) * 3.0 - 1.0;
}

float segmentDistance(vec2 p, vec2 a, vec2 b) {
    vec2 ab = b - a;
    float denominator = max(dot(ab, ab), 0.000001);
    float t = clamp(dot(p - a, ab) / denominator, 0.0, 1.0);
    return length(p - (a + ab * t));
}

float lowerBound(float x) {
    float lo = 0.0;
    float hi = pointCount;
    for (int i = 0; i < 14; i++) {
		if (lo >= hi) {
			break;
		}
        float mid = floor((lo + hi) * 0.5);
        if (mid >= hi) {
            continue;
        }
        if (pointAt(mid).x < x) {
            lo = mid + 1.0;
        } else {
            hi = mid;
        }
    }
    return min(lo, pointCount - 1.0);
}

void main() {
    if (pointCount < 1.0) {
        discard;
    }
    vec2 size = vec2(bounds[2] - bounds[0], bounds[3] - bounds[1]);
    vec2 origin = vec2(bounds[0], frame.y - bounds[3]);
    vec2 uv = (gl_FragCoord.xy - origin) / size;
    vec2 pixel = uv * size;
    float index = lowerBound(uv.x);
    float distancePx = 1000000.0;

    if (drawMode < 0.5) {
        if (pointCount == 1.0) {
            distancePx = length(pixel - pointAt(0.0) * size);
        } else {
            float left = max(index - 1.0, 0.0);
            float right = min(index, pointCount - 1.0);
            distancePx = segmentDistance(pixel, pointAt(left) * size, pointAt(right) * size);
            if (right + 1.0 < pointCount) {
                distancePx = min(distancePx, segmentDistance(pixel, pointAt(right) * size, pointAt(right + 1.0) * size));
            }
        }
    } else if (drawMode < 1.5) {
        for (int offset = -2; offset <= 2; offset++) {
            float candidate = clamp(index + float(offset), 0.0, pointCount - 1.0);
            distancePx = min(distancePx, length(pixel - pointAt(candidate) * size));
        }
    } else {
        for (int offset = -1; offset <= 1; offset++) {
            float candidate = clamp(index + float(offset), 0.0, pointCount - 1.0);
            vec2 value = pointAt(candidate);
            distancePx = min(distancePx, segmentDistance(pixel, vec2(value.x, stemBase) * size, value * size));
            distancePx = min(distancePx, length(pixel - value * size));
        }
    }

    float radius = drawMode < 0.5 ? max(lineWidth * 0.5, 0.5) : pointRadius;
    if (drawMode > 1.5) {
        radius = max(radius, lineWidth * 0.5);
    }
    float alpha = 1.0 - smoothstep(radius, radius + 1.0, distancePx);
    if (alpha <= 0.0) {
        discard;
    }
    gl_FragColor = vec4(colorR, colorG, colorB, colorA * alpha);
}
`

func newSeriesShader(slot int) *canvas.Shader {
	// Fyne кэширует и program, и его texture bindings по Name. Уникальное имя
	// слота не даёт сериям вытеснять текстуры друг друга на каждом draw call.
	name := fmt.Sprintf("gnssDeviceSDKPlotSeriesV1Slot%d", slot)
	shader := canvas.NewShader(name, []byte(plotShaderSource), []byte(plotShaderSourceES))
	shader.Textures = map[string]image.Image{"points": image.NewRGBA(image.Rect(0, 0, 1, 1))}
	shader.Uniforms = make(map[string]float32, 11)
	return shader
}

func configureSeriesShader(shader *canvas.Shader, series Series, view axisRange, fallback color.Color) {
	points := normalizedSeriesPoints(series.Points, view)
	texture := encodePoints(points)
	r, g, b, a := rgbaFloats(series.Color, fallback)
	shader.Textures["points"] = texture
	shader.Uniforms["pointCount"] = float32(len(points))
	shader.Uniforms["textureWidth"] = float32(texture.Bounds().Dx())
	shader.Uniforms["drawMode"] = float32(series.Mode)
	shader.Uniforms["lineWidth"] = series.LineWidth
	shader.Uniforms["pointRadius"] = series.PointRadius
	shader.Uniforms["stemBase"] = float32(clampCoordinate((0 - view.yMin) / (view.yMax - view.yMin)))
	shader.Uniforms["colorR"] = r
	shader.Uniforms["colorG"] = g
	shader.Uniforms["colorB"] = b
	shader.Uniforms["colorA"] = a
	shader.Refresh()
}

func normalizedSeriesPoints(points []Point, view axisRange) []Point {
	if len(points) == 0 {
		return nil
	}
	xWidth := view.xMax - view.xMin
	yWidth := view.yMax - view.yMin
	result := make([]Point, 0, len(points))
	for _, point := range points {
		result = append(result, Point{
			X: clampCoordinate((point.X - view.xMin) / xWidth),
			Y: clampCoordinate((point.Y - view.yMin) / yWidth),
		})
	}
	return result
}

func clampCoordinate(value float64) float64 {
	return math.Max(-1, math.Min(2, value))
}

func encodePoints(points []Point) *image.RGBA {
	width := len(points)
	if width == 0 {
		width = 1
	}
	encoded := image.NewRGBA(image.Rect(0, 0, width, 1))
	for index, point := range points {
		x := encodeCoordinate(point.X)
		y := encodeCoordinate(point.Y)
		offset := encoded.PixOffset(index, 0)
		encoded.Pix[offset] = byte(x >> 8)
		encoded.Pix[offset+1] = byte(x)
		encoded.Pix[offset+2] = byte(y >> 8)
		encoded.Pix[offset+3] = byte(y)
	}
	return encoded
}

func encodeCoordinate(value float64) uint16 {
	normalized := (clampCoordinate(value) + encodedCoordinateOffset) / encodedCoordinateScale
	return uint16(math.Round(normalized * 65535))
}

func rgbaFloats(configured color.Color, fallback color.Color) (float32, float32, float32, float32) {
	if configured == nil {
		configured = fallback
	}
	r, g, b, a := configured.RGBA()
	return float32(r) / 65535, float32(g) / 65535, float32(b) / 65535, float32(a) / 65535
}
