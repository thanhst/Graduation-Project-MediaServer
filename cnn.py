import numpy as np

# ------------------ Helper functions ------------------

def softmax(x):
    exps = np.exp(x - np.max(x))  # tránh overflow
    return exps / np.sum(exps, axis=1, keepdims=True)

def cross_entropy(pred, true_labels):
    m = true_labels.shape[0]
    log_likelihood = -np.log(pred[range(m), true_labels])
    loss = np.sum(log_likelihood) / m
    return loss

def relu(x):
    return np.maximum(0, x)

def relu_derivative(x):
    return (x > 0).astype(float)

def maxpool2x2(x):
    (n, h, w, c) = x.shape
    out = np.zeros((n, h//2, w//2, c))
    for i in range(0, h, 2):
        for j in range(0, w, 2):
            out[:, i//2, j//2, :] = np.max(x[:, i:i+2, j:j+2, :], axis=(1,2))
    return out

def maxpool2x2_backward(dout, x):
    (n, h, w, c) = x.shape
    dx = np.zeros_like(x)
    for i in range(0, h, 2):
        for j in range(0, w, 2):
            patch = x[:, i:i+2, j:j+2, :]
            max_patch = np.max(patch, axis=(1,2), keepdims=True)
            mask = (patch == max_patch)
            dx[:, i:i+2, j:j+2, :] += mask * (dout[:, i//2, j//2, :])[:, None, None, :]
    return dx

# ------------------ Layer classes ------------------

class Conv2D:
    def __init__(self, num_filters, filter_size, input_channels):
        self.num_filters = num_filters
        self.filter_size = filter_size
        self.input_channels = input_channels
        self.filters = np.random.randn(num_filters, filter_size, filter_size, input_channels) * 0.1
        self.biases = np.zeros((num_filters, 1))
    
    def forward(self, x):
        self.last_input = x
        n, h, w, c = x.shape
        f = self.filter_size
        out_h = h - f + 1
        out_w = w - f + 1
        out = np.zeros((n, out_h, out_w, self.num_filters))
        for i in range(out_h):
            for j in range(out_w):
                patch = x[:, i:i+f, j:j+f, :]
                for k in range(self.num_filters):
                    out[:, i, j, k] = np.sum(patch * self.filters[k], axis=(1,2,3)) + self.biases[k]
        return out
    
    def backward(self, d_out, learning_rate):
        x = self.last_input
        n, h, w, c = x.shape
        f = self.filter_size
        d_filters = np.zeros_like(self.filters)
        d_biases = np.zeros_like(self.biases)
        d_x = np.zeros_like(x)

        for i in range(h - f + 1):
            for j in range(w - f + 1):
                patch = x[:, i:i+f, j:j+f, :]
                for k in range(self.num_filters):
                    d_filters[k] += np.sum(patch * (d_out[:, i, j, k])[:, None, None, None], axis=0)
                for sample in range(n):
                    for k in range(self.num_filters):
                        d_x[sample, i:i+f, j:j+f, :] += self.filters[k] * d_out[sample, i, j, k]
        for k in range(self.num_filters):
            d_biases[k] = np.sum(d_out[:, :, :, k])

        self.filters -= learning_rate * d_filters
        self.biases -= learning_rate * d_biases

        return d_x

class Dense:
    def __init__(self, input_size, output_size):
        self.weights = np.random.randn(input_size, output_size) * 0.01
        self.bias = np.zeros((1, output_size))
    
    def forward(self, x):
        self.last_input = x
        return np.dot(x, self.weights) + self.bias
    
    def backward(self, d_out, learning_rate):
        d_weights = np.dot(self.last_input.T, d_out)
        d_bias = np.sum(d_out, axis=0, keepdims=True)
        d_input = np.dot(d_out, self.weights.T)
        
        self.weights -= learning_rate * d_weights
        self.bias -= learning_rate * d_bias
        return d_input

# ------------------ Model ------------------

class SimpleCNN:
    def __init__(self):
        self.conv = Conv2D(num_filters=8, filter_size=3, input_channels=1)
        self.fc = Dense(8*13*13, 7)  # Ví dụ ảnh 28x28 sau pooling còn 13x13
    
    def forward(self, x):
        x = self.conv.forward(x)
        x = relu(x)
        x = maxpool2x2(x)
        x = x.reshape(x.shape[0], -1)  # flatten
        x = self.fc.forward(x)
        return softmax(x)
    
    def backward(self, d_out, learning_rate):
        d_out = self.fc.backward(d_out, learning_rate)
        d_out = d_out.reshape(-1, 13, 13, 8)
        d_out = maxpool2x2_backward(d_out, relu(self.conv.last_input))
        d_out = relu_derivative(self.conv.last_input) * d_out
        self.conv.backward(d_out, learning_rate)
    
    def train(self, x_train, y_train, epochs, learning_rate):
        for epoch in range(epochs):
            preds = self.forward(x_train)
            loss = cross_entropy(preds, y_train)
            print(f"Epoch {epoch+1}, Loss: {loss}")
            grad = preds
            grad[range(x_train.shape[0]), y_train] -= 1
            grad /= x_train.shape[0]
            self.backward(grad, learning_rate)

